package usage

import "testing"

func TestEstimateTokens_IsDeterministic(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
	if got := EstimateTokens("abcd"); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	if got := EstimateTokens("abcde"); got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}
}

func TestTotals_AddCall(t *testing.T) {
	var totals Totals
	totals.AddCall(Call{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3, CostUSD: 0.5})
	totals.AddCall(Call{PromptTokens: -1, CompletionTokens: -1, TotalTokens: -1, CostUSD: -2})
	if totals.Calls != 2 {
		t.Fatalf("expected calls=2, got %+v", totals)
	}
	if totals.PromptTokens != 1 || totals.CompletionTokens != 2 || totals.TotalTokens != 3 {
		t.Fatalf("unexpected tokens: %+v", totals)
	}
	if totals.CostUSD != 0.5 {
		t.Fatalf("unexpected cost: %+v", totals)
	}
}

