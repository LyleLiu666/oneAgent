package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

func TestStreamUsageAccumulator_ReplacesCurrentCallUsageAndAccumulatesAcrossCalls(t *testing.T) {
	t.Parallel()

	var acc streamUsageAccumulator

	acc.StartCall()
	total := acc.UpdateCurrentCall(llm.UsageInfo{
		InputTokens:  100,
		OutputTokens: 12,
		TotalTokens:  112,
		CachedTokens: 60,
	})
	if total.InputTokens != 100 || total.OutputTokens != 12 || total.TotalTokens != 112 || total.CachedTokens != 60 {
		t.Fatalf("unexpected first total: %+v", total)
	}

	total = acc.UpdateCurrentCall(llm.UsageInfo{
		InputTokens:  120,
		OutputTokens: 15,
		TotalTokens:  135,
		CachedTokens: 75,
	})
	if total.InputTokens != 120 || total.OutputTokens != 15 || total.TotalTokens != 135 || total.CachedTokens != 75 {
		t.Fatalf("same-call update should replace prior usage, got %+v", total)
	}

	acc.StartCall()
	total = acc.UpdateCurrentCall(llm.UsageInfo{
		InputTokens:  30,
		OutputTokens: 5,
		TotalTokens:  35,
		CachedTokens: 0,
	})
	if total.InputTokens != 150 || total.OutputTokens != 20 || total.TotalTokens != 170 || total.CachedTokens != 75 {
		t.Fatalf("unexpected accumulated total: %+v", total)
	}
}

func TestBuildChatUsageEventPayload_IncludesCacheMetrics(t *testing.T) {
	t.Parallel()

	encoded, ok := buildChatUsageEventPayload(llm.UsageInfo{
		InputTokens:  100,
		OutputTokens: 12,
		TotalTokens:  112,
		CachedTokens: 75,
	})
	if !ok {
		t.Fatal("expected usage payload to be emitted")
	}

	var payload chatUsageEventPayload
	if err := json.Unmarshal([]byte(encoded), &payload); err != nil {
		t.Fatalf("unmarshal usage payload: %v", err)
	}

	if payload.ResponseTokens != 12 {
		t.Fatalf("expected response_tokens=12, got %d", payload.ResponseTokens)
	}
	if payload.InputTokens != 100 {
		t.Fatalf("expected input_tokens=100, got %d", payload.InputTokens)
	}
	if payload.CachedTokens != 75 {
		t.Fatalf("expected cached_tokens=75, got %d", payload.CachedTokens)
	}
	if payload.CacheHitRatio != 0.75 {
		t.Fatalf("expected cache_hit_ratio=0.75, got %v", payload.CacheHitRatio)
	}
}

func TestBuildKVCacheTrace_FormatsHumanReadableRatio(t *testing.T) {
	t.Parallel()

	got := buildKVCacheTrace(llm.UsageInfo{
		InputTokens:  9021,
		CachedTokens: 8832,
	})
	want := "KV cache: cached 8832/9021 prompt tokens (97.9%)"
	if got != want {
		t.Fatalf("unexpected trace: got %q want %q", got, want)
	}
}

func TestStreamTokenProgress_StopsApproxBroadcastAfterExactUsage(t *testing.T) {
	t.Parallel()

	var progress streamTokenProgress

	payload, ok := progress.AddToken("hello world", time.Unix(10, 0))
	if !ok {
		t.Fatal("expected approximate usage payload before exact usage")
	}
	if payload != `{"response_tokens": 3}` {
		t.Fatalf("unexpected approximate payload: %q", payload)
	}

	progress.MarkExactUsage()

	if payload, ok := progress.AddToken("more text", time.Unix(11, 0)); ok {
		t.Fatalf("expected no approximate payload after exact usage, got %q", payload)
	}
}
