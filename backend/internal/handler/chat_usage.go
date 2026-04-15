package handler

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
	"unicode/utf8"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

type streamUsageAccumulator struct {
	currentCall int
	perCall     map[int]llm.UsageInfo
}

func (a *streamUsageAccumulator) StartCall() {
	if a.perCall == nil {
		a.perCall = make(map[int]llm.UsageInfo)
	}
	a.currentCall++
}

func (a *streamUsageAccumulator) UpdateCurrentCall(usage llm.UsageInfo) llm.UsageInfo {
	if a.perCall == nil {
		a.perCall = make(map[int]llm.UsageInfo)
	}
	if a.currentCall <= 0 {
		a.currentCall = 1
	}
	a.perCall[a.currentCall] = usage
	return a.Total()
}

func (a *streamUsageAccumulator) Total() llm.UsageInfo {
	var total llm.UsageInfo
	for _, usage := range a.perCall {
		total.InputTokens += usage.InputTokens
		total.OutputTokens += usage.OutputTokens
		total.TotalTokens += usage.TotalTokens
		total.CachedTokens += usage.CachedTokens
	}
	return total
}

type chatUsageEventPayload struct {
	ResponseTokens int     `json:"response_tokens,omitempty"`
	InputTokens    int     `json:"input_tokens,omitempty"`
	OutputTokens   int     `json:"output_tokens,omitempty"`
	TotalTokens    int     `json:"total_tokens,omitempty"`
	CachedTokens   int     `json:"cached_tokens,omitempty"`
	CacheHitRatio  float64 `json:"cache_hit_ratio,omitempty"`
}

func buildChatUsageEventPayload(usage llm.UsageInfo) (string, bool) {
	if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.TotalTokens == 0 && usage.CachedTokens == 0 {
		return "", false
	}

	payload := chatUsageEventPayload{
		ResponseTokens: usage.OutputTokens,
		InputTokens:    usage.InputTokens,
		OutputTokens:   usage.OutputTokens,
		TotalTokens:    usage.TotalTokens,
		CachedTokens:   usage.CachedTokens,
	}
	if usage.InputTokens > 0 {
		payload.CacheHitRatio = normalizedCacheHitRatio(usage)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

func normalizedCacheHitRatio(usage llm.UsageInfo) float64 {
	if usage.InputTokens <= 0 {
		return 0
	}
	ratio := float64(usage.CachedTokens) / float64(usage.InputTokens)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return math.Round(ratio*10000) / 10000
}

func buildKVCacheTrace(usage llm.UsageInfo) string {
	if usage.InputTokens <= 0 {
		return ""
	}
	return fmt.Sprintf(
		"KV cache: cached %d/%d prompt tokens (%.1f%%)",
		usage.CachedTokens,
		usage.InputTokens,
		normalizedCacheHitRatio(usage)*100,
	)
}

type streamTokenProgress struct {
	runeCount           int
	lastBroadcastTokens int
	lastBroadcast       time.Time
	exactUsageObserved  bool
}

func (p *streamTokenProgress) MarkExactUsage() {
	p.exactUsageObserved = true
}

func (p *streamTokenProgress) AddToken(token string, now time.Time) (string, bool) {
	if p.exactUsageObserved {
		return "", false
	}

	p.runeCount += utf8.RuneCountInString(token)
	approxTokens := (p.runeCount + 3) / 4
	if approxTokens <= 0 || approxTokens == p.lastBroadcastTokens {
		return "", false
	}

	if approxTokens%5 != 0 && now.Sub(p.lastBroadcast) <= 100*time.Millisecond {
		return "", false
	}

	p.lastBroadcastTokens = approxTokens
	p.lastBroadcast = now
	return fmt.Sprintf(`{"response_tokens": %d}`, approxTokens), true
}
