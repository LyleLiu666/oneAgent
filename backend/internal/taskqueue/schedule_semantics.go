package taskqueue

import (
	"fmt"
	"strings"
	"time"
)

const (
	ScheduleMisfireOneShot = "one_shot"
	ScheduleMisfireCatchUp = "catch_up"
	ScheduleMisfireSkip    = "skip"
)

func normalizeMisfirePolicy(policy string) string {
	p := strings.TrimSpace(strings.ToLower(policy))
	switch p {
	case "", "one_shot", "one-shot":
		return ScheduleMisfireOneShot
	case "catch_up", "catch-up", "catchup":
		return ScheduleMisfireCatchUp
	case "skip":
		return ScheduleMisfireSkip
	default:
		return ScheduleMisfireOneShot
	}
}

func scheduleTriggerKey(scheduleID string, windowStart time.Time) string {
	return fmt.Sprintf("%s:%d", strings.TrimSpace(scheduleID), windowStart.UTC().Unix())
}

func scheduleTaskID(scheduleID string, windowStart time.Time) string {
	return fmt.Sprintf("schedule_%s_%d", sanitizeScheduleID(scheduleID), windowStart.UTC().Unix())
}

func sanitizeScheduleID(scheduleID string) string {
	id := strings.TrimSpace(scheduleID)
	if id == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(id))
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "unknown"
	}
	return out
}

func dueWindowsForSchedule(sc Schedule, now time.Time, maxCatchUp int) ([]time.Time, time.Time, bool) {
	now = now.UTC()
	interval := time.Duration(sc.EverySeconds) * time.Second
	if interval <= 0 {
		return nil, time.Time{}, false
	}

	base := sc.NextRunAt.UTC()
	if base.IsZero() {
		base = now
	}
	if now.Before(base) {
		return nil, time.Time{}, false
	}

	lateness := now.Sub(base)
	missedWindows := int(lateness / interval)
	if missedWindows < 0 {
		missedWindows = 0
	}

	policy := normalizeMisfirePolicy(sc.MisfirePolicy)

	// Misfire policy only kicks in once we've missed at least one full interval.
	if policy == ScheduleMisfireSkip && missedWindows >= 1 {
		nextAfter := base.Add(time.Duration(missedWindows+1) * interval)
		return nil, nextAfter, true
	}

	if policy == ScheduleMisfireCatchUp {
		dueCount := missedWindows + 1
		if maxCatchUp > 0 && dueCount > maxCatchUp {
			dueCount = maxCatchUp
		}

		windows := make([]time.Time, 0, dueCount)
		for i := 0; i < dueCount; i++ {
			windows = append(windows, base.Add(time.Duration(i)*interval))
		}
		nextAfter := base.Add(time.Duration(dueCount) * interval)
		return windows, nextAfter, true
	}

	// One-shot: enqueue a single run corresponding to the latest due window.
	windowStart := base.Add(time.Duration(missedWindows) * interval)
	nextAfter := base.Add(time.Duration(missedWindows+1) * interval)
	return []time.Time{windowStart}, nextAfter, true
}
