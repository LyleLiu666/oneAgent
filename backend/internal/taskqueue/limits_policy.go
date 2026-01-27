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
}

const (
	defaultMaxSteps          = 2000
	defaultMaxRuntimeSeconds = 6 * 60 * 60
)

func LimitsPolicyFromEnv() LimitsPolicy {
	p := LimitsPolicy{
		DefaultMaxSteps:          defaultMaxSteps,
		DefaultMaxRuntimeSeconds: defaultMaxRuntimeSeconds,
		MaxStepsCap:              0,
		MaxRuntimeSecondsCap:     0,
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

	if p.MaxStepsCap > 0 && out.MaxSteps > p.MaxStepsCap {
		out.MaxSteps = p.MaxStepsCap
	}
	if p.MaxRuntimeSecondsCap > 0 && out.MaxRuntimeSeconds > p.MaxRuntimeSecondsCap {
		out.MaxRuntimeSeconds = p.MaxRuntimeSecondsCap
	}

	// Absolute minimums to avoid "instant cancel" surprises.
	if out.MaxSteps < 1 {
		out.MaxSteps = 1
	}
	if out.MaxRuntimeSeconds < 1 {
		out.MaxRuntimeSeconds = 1
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
