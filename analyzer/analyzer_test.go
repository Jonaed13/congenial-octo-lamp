package analyzer

import (
	"testing"
)

func TestExtractWinrate(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected float64
	}{
		{
			name:     "Valid winrate",
			html:     `<div><h3>Win Rate</h3><p class="text-2xl font-bold">65.50%</p></div>`,
			expected: 65.50,
		},
		{
			name:     "Winrate with extra classes",
			html:     `<h3 class="text-sm">Win Rate</h3><p class="text-green-500 text-2xl">82.33%</p>`,
			expected: 82.33,
		},
		{
			name:     "No winrate found",
			html:     `<div><h3>Other Metric</h3><p>50%</p></div>`,
			expected: 0,
		},
		{
			name:     "Low winrate",
			html:     `<h3>Win Rate</h3><p class="text-2xl">15.75%</p>`,
			expected: 15.75,
		},
		{
			name:     "Perfect winrate",
			html:     `<h3>Win Rate</h3><p class="text-2xl font-semibold">100.00%</p>`,
			expected: 100.00,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractWinrate(tt.html)
			if result != tt.expected {
				t.Errorf("extractWinrate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractRealizedPnL(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected float64
	}{
		{
			name:     "Positive PnL",
			html:     `<p>Realized</p><p>$1,234.56 <span class="text-green-500">(245.80%)</span></p>`,
			expected: 245.80,
		},
		{
			name:     "Negative PnL",
			html:     `<p>Realized</p><p>-$500.00 <span class="text-red-500">(-25.50%)</span></p>`,
			expected: -25.50,
		},
		{
			name:     "Large positive PnL",
			html:     `<p>Realized</p><p>$10,000.00 <span>(1500.75%)</span></p>`,
			expected: 1500.75,
		},
		{
			name:     "Small positive PnL",
			html:     `<p>Realized</p><p>$50.25 <span>(5.25%)</span></p>`,
			expected: 5.25,
		},
		{
			name:     "No PnL found",
			html:     `<div><p>Other Metric</p><p>$100</p></div>`,
			expected: 0,
		},
		{
			name:     "Zero PnL",
			html:     `<p>Realized</p><p>$0.00 <span>(0.00%)</span></p>`,
			expected: 0.00,
		},
		{
			name:     "PnL with decimal precision",
			html:     `<p>Realized</p><p>$2,567.89 <span class="text-green-600">(356.123%)</span></p>`,
			expected: 356.123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRealizedPnL(tt.html)
			if result != tt.expected {
				t.Errorf("extractRealizedPnL() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractWinrateEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected float64
	}{
		{
			name:     "Case insensitive Win Rate",
			html:     `<h3>win rate</h3><p class="text-2xl">75.00%</p>`,
			expected: 75.00,
		},
		{
			name:     "Multiple occurrences - should match first",
			html:     `<h3>Win Rate</h3><p class="text-2xl">60.00%</p><h3>Win Rate</h3><p class="text-2xl">70.00%</p>`,
			expected: 60.00,
		},
		{
			name:     "Empty string",
			html:     ``,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractWinrate(tt.html)
			if result != tt.expected {
				t.Errorf("extractWinrate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractRealizedPnLEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected float64
	}{
		{
			name:     "Case insensitive Realized",
			html:     `<p>realized</p><p>$100.00 <span>(50.00%)</span></p>`,
			expected: 50.00,
		},
		{
			name:     "Multiple occurrences - should match first",
			html:     `<p>Realized</p><p>$100 <span>(50%)</span></p><p>Realized</p><p>$200 <span>(100%)</span></p>`,
			expected: 50.00,
		},
		{
			name:     "Empty string",
			html:     ``,
			expected: 0,
		},
		{
			name:     "Large negative PnL",
			html:     `<p>Realized</p><p>-$5,000.00 <span class="text-red-600">(-99.99%)</span></p>`,
			expected: -99.99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRealizedPnL(tt.html)
			if result != tt.expected {
				t.Errorf("extractRealizedPnL() = %v, want %v", result, tt.expected)
			}
		})
	}
}
