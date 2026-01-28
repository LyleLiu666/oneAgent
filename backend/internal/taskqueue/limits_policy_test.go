package taskqueue

import "testing"

func TestResolveLimits_Defaults(t *testing.T) {
	t.Setenv("ONEAGENT_TASK_DEFAULT_MAX_STEPS", "")
	t.Setenv("ONEAGENT_TASK_DEFAULT_MAX_RUNTIME_SECONDS", "")
	t.Setenv("ONEAGENT_TASK_MAX_STEPS_CAP", "")
	t.Setenv("ONEAGENT_TASK_MAX_RUNTIME_SECONDS_CAP", "")
	t.Setenv("ONEAGENT_TASK_DEFAULT_MAX_TOTAL_TOKENS", "")
	t.Setenv("ONEAGENT_TASK_DEFAULT_MAX_COST_USD", "")
	t.Setenv("ONEAGENT_TASK_MAX_TOTAL_TOKENS_CAP", "")
	t.Setenv("ONEAGENT_TASK_MAX_COST_USD_CAP", "")

	got := ResolveLimits(Limits{})
	if got.MaxSteps != defaultMaxSteps {
		t.Fatalf("expected default max steps=%d, got %+v", defaultMaxSteps, got)
	}
	if got.MaxRuntimeSeconds != defaultMaxRuntimeSeconds {
		t.Fatalf("expected default max runtime=%d, got %+v", defaultMaxRuntimeSeconds, got)
	}
	if got.MaxTotalTokens != 0 || got.MaxCostUSD != 0 {
		t.Fatalf("expected no default budgets, got %+v", got)
	}
}

func TestResolveLimits_UsesEnvOverridesAndCaps(t *testing.T) {
	t.Setenv("ONEAGENT_TASK_DEFAULT_MAX_STEPS", "10")
	t.Setenv("ONEAGENT_TASK_DEFAULT_MAX_RUNTIME_SECONDS", "20")
	t.Setenv("ONEAGENT_TASK_MAX_STEPS_CAP", "11")
	t.Setenv("ONEAGENT_TASK_MAX_RUNTIME_SECONDS_CAP", "21")
	t.Setenv("ONEAGENT_TASK_DEFAULT_MAX_TOTAL_TOKENS", "100")
	t.Setenv("ONEAGENT_TASK_DEFAULT_MAX_COST_USD", "12.5")
	t.Setenv("ONEAGENT_TASK_MAX_TOTAL_TOKENS_CAP", "110")
	t.Setenv("ONEAGENT_TASK_MAX_COST_USD_CAP", "13.5")

	got := ResolveLimits(Limits{})
	if got.MaxSteps != 10 || got.MaxRuntimeSeconds != 20 || got.MaxTotalTokens != 100 || got.MaxCostUSD != 12.5 {
		t.Fatalf("expected env defaults, got %+v", got)
	}

	// Requested above cap should be clamped.
	got = ResolveLimits(Limits{MaxSteps: 999, MaxRuntimeSeconds: 999, MaxTotalTokens: 999, MaxCostUSD: 999})
	if got.MaxSteps != 11 || got.MaxRuntimeSeconds != 21 || got.MaxTotalTokens != 110 || got.MaxCostUSD != 13.5 {
		t.Fatalf("expected caps, got %+v", got)
	}
}
