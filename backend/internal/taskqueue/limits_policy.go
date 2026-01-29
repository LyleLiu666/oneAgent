package taskqueue

import (
	"os"
	"strconv"
	"strings"
)

type LimitsPolicy struct {
	DefaultMaxSteps          int
	DefaultMaxRuntimeSeconds int

	MaxStepsCap          int
	MaxRuntimeSecondsCap int

	DefaultMaxTotalTokens int
	DefaultMaxCostUSD     float64

	MaxTotalTokensCap int
	MaxCostUSDCap     float64

	DefaultMaxAutoAttempts int
	MaxAutoAttemptsCap     int
}

const (
	defaultMaxSteps          = 2000
	defaultMaxRuntimeSeconds = 6 * 60 * 60
	defaultMaxAutoAttempts   = 3
)

func LimitsPolicyFromEnv() LimitsPolicy {
	p := LimitsPolicy{
		DefaultMaxSteps:          defaultMaxSteps,
		DefaultMaxRuntimeSeconds: defaultMaxRuntimeSeconds,
		MaxStepsCap:              0,
		MaxRuntimeSecondsCap:     0,
		DefaultMaxTotalTokens:    0,
		DefaultMaxCostUSD:        0,
		MaxTotalTokensCap:        0,
		MaxCostUSDCap:            0,
		DefaultMaxAutoAttempts:   defaultMaxAutoAttempts,
		MaxAutoAttemptsCap:       0,
	}

	if v := envInt("ONEAGENT_TASK_DEFAULT_MAX_STEPS"); v > 0 {
		p.DefaultMaxSteps = v
	}
	if v := envInt("ONEAGENT_TASK_DEFAULT_MAX_RUNTIME_SECONDS"); v > 0 {
		p.DefaultMaxRuntimeSeconds = v
	}
	if v := envInt("ONEAGENT_TASK_MAX_STEPS_CAP"); v > 0 {
		p.MaxStepsCap = v
	}
	if v := envInt("ONEAGENT_TASK_MAX_RUNTIME_SECONDS_CAP"); v > 0 {
		p.MaxRuntimeSecondsCap = v
	}
	if v := envInt("ONEAGENT_TASK_DEFAULT_MAX_TOTAL_TOKENS"); v > 0 {
		p.DefaultMaxTotalTokens = v
	}
	if v := envFloat("ONEAGENT_TASK_DEFAULT_MAX_COST_USD"); v > 0 {
		p.DefaultMaxCostUSD = v
	}
	if v := envInt("ONEAGENT_TASK_MAX_TOTAL_TOKENS_CAP"); v > 0 {
		p.MaxTotalTokensCap = v
	}
	if v := envFloat("ONEAGENT_TASK_MAX_COST_USD_CAP"); v > 0 {
		p.MaxCostUSDCap = v
	}
	if v, ok := envIntOptional("ONEAGENT_TASK_DEFAULT_MAX_AUTO_ATTEMPTS"); ok && v >= 0 {
		p.DefaultMaxAutoAttempts = v
	}
	if v := envInt("ONEAGENT_TASK_MAX_AUTO_ATTEMPTS_CAP"); v > 0 {
		p.MaxAutoAttemptsCap = v
	}

	return p
}

func ApplyLimitsPolicy(in Limits, p LimitsPolicy) Limits {
	out := in

	if out.MaxSteps <= 0 {
		out.MaxSteps = p.DefaultMaxSteps
	}
	if out.MaxRuntimeSeconds <= 0 {
		out.MaxRuntimeSeconds = p.DefaultMaxRuntimeSeconds
	}
	if out.MaxTotalTokens <= 0 && p.DefaultMaxTotalTokens > 0 {
		out.MaxTotalTokens = p.DefaultMaxTotalTokens
	}
	if out.MaxCostUSD <= 0 && p.DefaultMaxCostUSD > 0 {
		out.MaxCostUSD = p.DefaultMaxCostUSD
	}
	if out.MaxAutoAttempts < 0 {
		out.MaxAutoAttempts = 0
	}
	if out.MaxAutoAttempts == 0 && p.DefaultMaxAutoAttempts >= 0 {
		out.MaxAutoAttempts = p.DefaultMaxAutoAttempts
	}

	if p.MaxStepsCap > 0 && out.MaxSteps > p.MaxStepsCap {
		out.MaxSteps = p.MaxStepsCap
	}
	if p.MaxRuntimeSecondsCap > 0 && out.MaxRuntimeSeconds > p.MaxRuntimeSecondsCap {
		out.MaxRuntimeSeconds = p.MaxRuntimeSecondsCap
	}
	if p.MaxTotalTokensCap > 0 && out.MaxTotalTokens > p.MaxTotalTokensCap {
		out.MaxTotalTokens = p.MaxTotalTokensCap
	}
	if p.MaxCostUSDCap > 0 && out.MaxCostUSD > p.MaxCostUSDCap {
		out.MaxCostUSD = p.MaxCostUSDCap
	}
	if p.MaxAutoAttemptsCap > 0 && out.MaxAutoAttempts > p.MaxAutoAttemptsCap {
		out.MaxAutoAttempts = p.MaxAutoAttemptsCap
	}

	// Absolute minimums to avoid "instant cancel" surprises.
	if out.MaxSteps < 1 {
		out.MaxSteps = 1
	}
	if out.MaxRuntimeSeconds < 1 {
		out.MaxRuntimeSeconds = 1
	}
	if out.MaxTotalTokens < 0 {
		out.MaxTotalTokens = 0
	}
	if out.MaxCostUSD < 0 {
		out.MaxCostUSD = 0
	}
	if out.MaxAutoAttempts < 0 {
		out.MaxAutoAttempts = 0
	}

	return out
}

func ResolveLimits(in Limits) Limits {
	return ApplyLimitsPolicy(in, LimitsPolicyFromEnv())
}

func envInt(key string) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return v
}

func envIntOptional(key string) (int, bool) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return v, true
}

func envFloat(key string) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return v
}
