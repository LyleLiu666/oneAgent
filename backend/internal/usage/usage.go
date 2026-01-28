package usage

import "unicode/utf8"

// Totals is a best-effort, provider-agnostic usage aggregate.
// Values are approximate unless populated from provider-reported usage.
type Totals struct {
	Calls            int     `json:"calls,omitempty"`
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	TotalTokens      int     `json:"total_tokens,omitempty"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
}

type Call struct {
	Provider         string  `json:"provider,omitempty"`
	Model            string  `json:"model,omitempty"`
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	TotalTokens      int     `json:"total_tokens,omitempty"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
}

func (t *Totals) AddCall(call Call) {
	if t == nil {
		return
	}
	t.Calls++
	t.PromptTokens += maxInt(0, call.PromptTokens)
	t.CompletionTokens += maxInt(0, call.CompletionTokens)
	t.TotalTokens += maxInt(0, call.TotalTokens)
	t.CostUSD += maxFloat(0, call.CostUSD)
}

// EstimateTokens provides a stable heuristic for "token-like units" without relying on a tokenizer.
// The mapping is intentionally coarse: ~4 runes ≈ 1 token.
func EstimateTokens(text string) int {
	r := utf8.RuneCountInString(text)
	if r <= 0 {
		return 0
	}
	return (r + 3) / 4
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

