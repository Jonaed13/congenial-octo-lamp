package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const AdminUserID = 7346780383

// isAdmin checks if the user is the authorized admin
func isAdmin(userID int64) bool {
	return userID == AdminUserID
}

// handleAdminCommand shows the main admin dashboard
func handleAdminCommand(bot *tgbotapi.BotAPI, chatID int64) {
	if !isAdmin(chatID) {
		return // Silent ignore for non-admins
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Manage User", "admin_manage_user"),
			tgbotapi.NewInlineKeyboardButtonData("📊 Global Stats", "admin_stats"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "🛡️ *Admin Dashboard*\n\nSelect an action:")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// handleAdminCallback processes admin button clicks
func handleAdminCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID
	data := callback.Data

	if !isAdmin(chatID) {
		return
	}

	if data == "admin_manage_user" {
		// Ask for User ID
		sessMu.Lock()
		sessions[chatID] = &UserSession{
			State:       "admin_awaiting_userid",
			RequestedAt: time.Now().Unix(),
		}
		sessMu.Unlock()
		send(bot, chatID, "👥 *Manage User*\n\nPlease send the **Telegram User ID** you want to manage:")
	} else if data == "admin_stats" {
		// Placeholder for stats
		send(bot, chatID, "📊 *Global Stats*\n\n(Coming Soon)")
	} else if strings.HasPrefix(data, "admin_set_plan:") {
		parts := strings.Split(data, ":")
		if len(parts) == 3 {
			targetUserID, _ := strconv.ParseInt(parts[1], 10, 64)
			planType := parts[2]
			handleAdminSetPlan(bot, chatID, targetUserID, planType)
		}
	} else if strings.HasPrefix(data, "admin_add_credits:") {
		targetUserID, _ := strconv.ParseInt(strings.TrimPrefix(data, "admin_add_credits:"), 10, 64)
		sessMu.Lock()
		sessions[chatID] = &UserSession{
			State:       "admin_awaiting_credits",
			RequestedAt: time.Now().Unix(),
			TempData:    map[string]interface{}{"target_user_id": targetUserID},
		}
		sessMu.Unlock()
		send(bot, chatID, fmt.Sprintf("➕ *Add Credits*\n\nEnter amount to add for User `%d`:", targetUserID))
	}
}

// handleAdminInput processes text input for admin states
func handleAdminInput(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	text := msg.Text

	sessMu.Lock()
	session, exists := sessions[chatID]
	sessMu.Unlock()

	if !exists {
		return
	}

	if session.State == "admin_awaiting_userid" {
		targetUserID, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			send(bot, chatID, "❌ Invalid User ID. Please enter a number.")
			return
		}
		showUserDetails(bot, chatID, targetUserID)
		// Clear state
		sessMu.Lock()
		delete(sessions, chatID)
		sessMu.Unlock()

	} else if session.State == "admin_awaiting_credits" {
		amount, err := strconv.Atoi(text)
		if err != nil {
			send(bot, chatID, "❌ Invalid amount. Please enter a number.")
			return
		}

		targetUserID := session.TempData["target_user_id"].(int64)

		// Update credits
		user, err := scanner.db.GetUser(targetUserID)
		if err != nil || user == nil {
			send(bot, chatID, "❌ User not found.")
			return
		}

		newCredits := user.Credits + amount
		if err := scanner.db.UpdateUserCredits(targetUserID, newCredits); err != nil {
			log.Printf("Error updating credits: %v", err)
			send(bot, chatID, "❌ Database error.")
			return
		}

		send(bot, chatID, fmt.Sprintf("✅ Added %d credits to User `%d`.\nNew Balance: %d", amount, targetUserID, newCredits))

		// Clear state
		sessMu.Lock()
		delete(sessions, chatID)
		sessMu.Unlock()
	}
}

// showUserDetails displays user info and action buttons
func showUserDetails(bot *tgbotapi.BotAPI, adminChatID, targetUserID int64) {
	user, err := scanner.db.GetUser(targetUserID)
	if err != nil {
		send(bot, adminChatID, "❌ Error retrieving user.")
		return
	}
	if user == nil {
		send(bot, adminChatID, "⚠️ User not found in database.")
		return
	}

	// Format Plan Info
	planInfo := "None"
	if user.PlanType == "credits_1000" {
		planInfo = fmt.Sprintf("💎 Credits (Bal: %d)", user.Credits)
	} else if user.PlanType == "trial_3day" {
		timeLeft := time.Until(time.Unix(user.TrialExpiresAt, 0))
		planInfo = fmt.Sprintf("⏳ Trial (Expires in %.1fh)", timeLeft.Hours())
	}

	text := fmt.Sprintf("👤 *User Details*\n\n"+
		"🆔 ID: `%d`\n"+
		"📅 Joined: %s\n"+
		"📋 Plan: %s\n\n"+
		"Select Action:",
		user.UserID,
		time.Unix(user.JoinedAt, 0).Format("2006-01-02"),
		planInfo)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💎 Set 1000 Credits", fmt.Sprintf("admin_set_plan:%d:credits_1000", targetUserID)),
			tgbotapi.NewInlineKeyboardButtonData("⏳ Set 3-Day Trial", fmt.Sprintf("admin_set_plan:%d:trial_3day", targetUserID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Add Credits", fmt.Sprintf("admin_add_credits:%d", targetUserID)),
		),
	)

	msg := tgbotapi.NewMessage(adminChatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// handleAdminSetPlan updates the user's plan
func handleAdminSetPlan(bot *tgbotapi.BotAPI, adminChatID, targetUserID int64, planType string) {
	var err error
	if planType == "credits_1000" {
		err = scanner.db.SetUserPlan(targetUserID, "credits_1000", 1000, 0)
		if err == nil {
			send(bot, adminChatID, fmt.Sprintf("✅ User `%d` set to **1000 Credits** plan.", targetUserID))
		}
	} else if planType == "trial_3day" {
		expiresAt := time.Now().Add(3 * 24 * time.Hour).Unix()
		err = scanner.db.SetUserPlan(targetUserID, "trial_3day", 0, expiresAt)
		if err == nil {
			send(bot, adminChatID, fmt.Sprintf("✅ User `%d` set to **3-Day Trial**.", targetUserID))
		}
	}

	if err != nil {
		log.Printf("Error setting plan: %v", err)
		send(bot, adminChatID, "❌ Error updating plan.")
	}
}
