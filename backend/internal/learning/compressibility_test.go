package learning

import (
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func TestApplyCompressibilityToScores(t *testing.T) {
	base := workledger.SuggestionScores{
		ScarcityScore: 0.5,
		EvidenceScore: 0.5,
		DepthScore:    0.9,
		TotalScore:    0,
	}

	got := applyCompressibilityToScores(base, CompressibilityVerdictEquivalent)
	if got.DepthScore != 0.3 {
		t.Fatalf("expected depth=0.3, got %v", got.DepthScore)
	}

	base.DepthScore = 0.1
	got = applyCompressibilityToScores(base, CompressibilityVerdictNotEquivalent)
	if got.DepthScore != 0.7 {
		t.Fatalf("expected depth=0.7, got %v", got.DepthScore)
	}
}
