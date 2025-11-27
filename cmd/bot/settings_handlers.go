package main

import (
	"fmt"
	"log"
	"solana-orchestrator/storage"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleSettings shows settings menu
func handleSettings(bot *tgbotapi.BotAPI, chatID int64) {
	settings, err := scanner.db.GetUserSettings(chatID)
	if err != nil {
		log.Printf("Error loading settings: %v", err)
		settings = &storage.UserSettings{SlippageBps: 500, JitoTipLamports: 10000}
	}

	message := "⚙️ *Settings*\n\n"
	message += fmt.Sprintf("📊 *Slippage:* %.1f%%\n", float64(settings.SlippageBps)/100)
	message += fmt.Sprintf("💎 *Jito Tip:* %.6f SOL\n", float64(settings.JitoTipLamports)/1e9)
	message += fmt.Sprintf("⚡ *Priority Fee:* %.6f SOL\n\n", float64(settings.PriorityFeeLamports)/1e9)
	message += "Click below to change settings:"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Change Slippage", "settings_slippage"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💎 Change Jito Tip", "settings_jito"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Change Priority Fee", "settings_priority"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🤖 Copy Trade Settings", "settings_copytrade"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "back_to_menu"),
		),
	)

	msgConfig := tgbotapi.NewMessage(chatID, message)
	msgConfig.ParseMode = "Markdown"
	msgConfig.ReplyMarkup = keyboard
	bot.Send(msgConfig)
}

// handleSettingsSlippage shows slippage options
func handleSettingsSlippage(bot *tgbotapi.BotAPI, chatID int64) {
	message := "📊 *Set Slippage*\n\n"
	message += "Choose your slippage tolerance:\n\n"
	message += "• Lower = less slippage, higher chance of failed trade\n"
	message += "• Higher = more slippage, better for volatile tokens"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("0.5%", "set_slip_50"),
			tgbotapi.NewInlineKeyboardButtonData("1%", "set_slip_100"),
			tgbotapi.NewInlineKeyboardButtonData("2%", "set_slip_200"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("5%", "set_slip_500"),
			tgbotapi.NewInlineKeyboardButtonData("10%", "set_slip_1000"),
			tgbotapi.NewInlineKeyboardButtonData("25%", "set_slip_2500"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("50%", "set_slip_5000"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "open_settings"),
		),
	)

	msgConfig := tgbotapi.NewMessage(chatID, message)
	msgConfig.ParseMode = "Markdown"
	msgConfig.ReplyMarkup = keyboard
	bot.Send(msgConfig)
}

// handleSetSlippage updates slippage setting
func handleSetSlippage(bot *tgbotapi.BotAPI, chatID int64, bps int) {
	err := scanner.db.UpdateSlippage(chatID, bps)
	if err != nil {
		sendError(bot, chatID, fmt.Sprintf("Error updating slippage: %v", err))
		return
	}

	send(bot, chatID, fmt.Sprintf("✅ Slippage set to %.1f%%", float64(bps)/100))
	handleSettings(bot, chatID)
}

// handleSettingsJito shows Jito tip options
func handleSettingsJito(bot *tgbotapi.BotAPI, chatID int64) {
	message := "💎 *Set Jito Tip*\n\n"
	message += "Higher tips = faster execution\n"
	message += "Lower tips = save on fees"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("0.00001 SOL", "set_jito_10000"),
			tgbotapi.NewInlineKeyboardButtonData("0.0001 SOL", "set_jito_100000"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("0.001 SOL", "set_jito_1000000"),
			tgbotapi.NewInlineKeyboardButtonData("0.01 SOL", "set_jito_10000000"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "open_settings"),
		),
	)

	msgConfig := tgbotapi.NewMessage(chatID, message)
	msgConfig.ParseMode = "Markdown"
	msgConfig.ReplyMarkup = keyboard
	bot.Send(msgConfig)
}

// handleSetJito updates Jito tip
func handleSetJito(bot *tgbotapi.BotAPI, chatID int64, lamports int64) {
	err := scanner.db.UpdateJitoTip(chatID, lamports)
	if err != nil {
		sendError(bot, chatID, fmt.Sprintf("Error updating Jito tip: %v", err))
		return
	}

	send(bot, chatID, fmt.Sprintf("✅ Jito tip set to %.6f SOL", float64(lamports)/1e9))
	handleSettings(bot, chatID)
}

// handleSettingsPriority shows Priority Fee options
func handleSettingsPriority(bot *tgbotapi.BotAPI, chatID int64) {
	message := "⚡ *Set Priority Fee*\n\n"
	message += "Fee paid to miners to prioritize your transaction.\n"
	message += "Higher fees = faster confirmation during congestion."

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("0.0001 SOL", "set_prio_100000"),
			tgbotapi.NewInlineKeyboardButtonData("0.001 SOL", "set_prio_1000000"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("0.005 SOL", "set_prio_5000000"),
			tgbotapi.NewInlineKeyboardButtonData("0.01 SOL", "set_prio_10000000"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "open_settings"),
		),
	)

	msgConfig := tgbotapi.NewMessage(chatID, message)
	msgConfig.ParseMode = "Markdown"
	msgConfig.ReplyMarkup = keyboard
	bot.Send(msgConfig)
}

// handleSetPriority updates Priority Fee
func handleSetPriority(bot *tgbotapi.BotAPI, chatID int64, lamports int64) {
	err := scanner.db.UpdatePriorityFee(chatID, lamports)
	if err != nil {
		sendError(bot, chatID, fmt.Sprintf("Error updating priority fee: %v", err))
		return
	}

	send(bot, chatID, fmt.Sprintf("✅ Priority fee set to %.6f SOL", float64(lamports)/1e9))
	handleSettings(bot, chatID)
}

// Helper to parse slippage from callback data
func parseSlippageCallback(data string) int {
	// Format: set_slip_XXX where XXX is bps
	if len(data) <= 9 {
		return 500 // default
	}
	bpsStr := data[9:]
	bps, err := strconv.Atoi(bpsStr)
	if err != nil {
		return 500
	}
	return bps
}

// Helper to parse Jito tip from callback data
func parseJitoCallback(data string) int64 {
	// Format: set_jito_XXX where XXX is lamports
	if len(data) <= 9 {
		return 10000 // default
	}
	lamportsStr := data[9:]
	lamports, err := strconv.ParseInt(lamportsStr, 10, 64)
	if err != nil {
		return 10000
	}
	return lamports
}

// Helper to parse Priority Fee from callback data
func parsePriorityCallback(data string) int64 {
	// Format: set_prio_XXX where XXX is lamports
	if len(data) <= 9 {
		return 100000 // default
	}
	lamportsStr := data[9:]
	lamports, err := strconv.ParseInt(lamportsStr, 10, 64)
	if err != nil {
		return 100000
	}
	return lamports
}

// handleSettingsCopyTrade shows copy trade settings
func handleSettingsCopyTrade(bot *tgbotapi.BotAPI, chatID int64) {
	settings, err := scanner.db.GetUserSettings(chatID)
	if err != nil {
		log.Printf("Error loading settings: %v", err)
		settings = &storage.UserSettings{}
	}

	status := "🔴 OFF"
	toggleAction := "toggle_copy_autobuy_on"
	if settings.CopyTradeAutoBuy {
		status = "🟢 ON"
		toggleAction = "toggle_copy_autobuy_off"
	}

	message := "🤖 *Copy Trade Settings*\n\n"
	message += fmt.Sprintf("🚀 *Auto-Buy:* %s\n", status)
	message += "_When ON, the bot will automatically execute trades when a target wallet trades._\n"
	message += "_When OFF, you will only receive alerts._\n\n"
	message += "⚠️ *Risk Warning:* Copy trading involves risk. Ensure you trust the target wallet."

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Toggle Auto-Buy: %s", status), toggleAction),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "open_settings"),
		),
	)

	msgConfig := tgbotapi.NewMessage(chatID, message)
	msgConfig.ParseMode = "Markdown"
	msgConfig.ReplyMarkup = keyboard
	bot.Send(msgConfig)
}

// handleToggleCopyTradeAutoBuy toggles auto-buy
func handleToggleCopyTradeAutoBuy(bot *tgbotapi.BotAPI, chatID int64, enable bool) {
	err := scanner.db.UpdateCopyTradeAutoBuy(chatID, enable)
	if err != nil {
		sendError(bot, chatID, fmt.Sprintf("Error updating setting: %v", err))
		return
	}
	handleSettingsCopyTrade(bot, chatID)
}
