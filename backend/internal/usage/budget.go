package usage

import (
	"fmt"
	"strings"
)

type BudgetExceededError struct {
	MaxTotalTokens int
	MaxCostUSD     float64
	Used           Totals
	Message        string
}

func (e *BudgetExceededError) Error() string {
	if e == nil {
		return "budget exceeded"
	}
	if strings.TrimSpace(e.Message) != "" {
		return strings.TrimSpace(e.Message)
	}
	if e.MaxTotalTokens > 0 && e.MaxCostUSD > 0 {
		return fmt.Sprintf("budget exceeded: tokens=%d/%d cost=$%.4f/$%.4f", e.Used.TotalTokens, e.MaxTotalTokens, e.Used.CostUSD, e.MaxCostUSD)
	}
	if e.MaxTotalTokens > 0 {
		return fmt.Sprintf("budget exceeded: tokens=%d/%d", e.Used.TotalTokens, e.MaxTotalTokens)
	}
	if e.MaxCostUSD > 0 {
		return fmt.Sprintf("budget exceeded: cost=$%.4f/$%.4f", e.Used.CostUSD, e.MaxCostUSD)
	}
	return "budget exceeded"
}

