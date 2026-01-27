package handler

import (
	"strings"
	"unicode"

	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

func computeSuggestionScores(store *workledger.Store, principalID, dayKey, title, draftSkill string, evidenceIDs []string) workledger.SuggestionScores {
	evidenceScore := scoreEvidence(len(evidenceIDs))
	depthScore := scoreDepth(draftSkill)
	scarcityScore := scoreScarcity(store, principalID, dayKey, title, draftSkill)
	total := 0.40*depthScore + 0.35*scarcityScore + 0.25*evidenceScore
	return workledger.SuggestionScores{
		ScarcityScore: scarcityScore,
		DepthScore:    depthScore,
		EvidenceScore: evidenceScore,
		TotalScore:    total,
	}
}

func scoreEvidence(n int) float64 {
	if n <= 0 {
		return 0
	}
	if n >= 3 {
		return 1
	}
	return float64(n) / 3.0
}

func scoreDepth(draft string) float64 {
	text := strings.ToLower(draft)
	hasWhen := strings.Contains(text, "when") || strings.Contains(text, "适用") || strings.Contains(text, "使用时机")
	hasWhenNot := strings.Contains(text, "when not") || strings.Contains(text, "不适用") || strings.Contains(text, "不要") || strings.Contains(text, "禁用")
	hasFailure := strings.Contains(text, "failure") || strings.Contains(text, "失败") || strings.Contains(text, "异常") || strings.Contains(text, "回滚") || strings.Contains(text, "resume")
	hasVerify := strings.Contains(text, "验收") || strings.Contains(text, "verify") || strings.Contains(text, "test") || strings.Contains(text, "报告") || strings.Contains(text, "evidence")

	stepCount := countNumberedSteps(text)

	score := 0.0
	if hasWhen {
		score += 0.2
	}
	if hasWhenNot {
		score += 0.2
	}
	if hasFailure {
		score += 0.2
	}
	if hasVerify {
		score += 0.2
	}
	switch {
	case stepCount >= 8:
		score += 0.2
	case stepCount >= 5:
		score += 0.15
	case stepCount >= 3:
		score += 0.1
	}
	if score > 1 {
		return 1
	}
	return score
}

func countNumberedSteps(text string) int {
	lines := strings.Split(text, "\n")
	n := 0
	for _, raw := range lines {
		line := strings.TrimLeftFunc(raw, unicode.IsSpace)
		if line == "" {
			continue
		}
		if len(line) >= 2 && line[0] >= '0' && line[0] <= '9' {
			i := 0
			for i < len(line) && line[i] >= '0' && line[i] <= '9' {
				i++
			}
			if i < len(line) && (line[i] == '.' || line[i] == ')') {
				n++
			}
		}
	}
	return n
}

func scoreScarcity(store *workledger.Store, principalID, dayKey, title, draft string) float64 {
	if store == nil {
		return 0.5
	}
	title = strings.TrimSpace(title)
	draft = strings.TrimSpace(draft)
	tokens := toTokenSet(title + " " + draft)
	if len(tokens) == 0 {
		return 0.5
	}

	existing, err := store.ListSuggestions(workledger.ListSuggestionsQuery{
		PrincipalID:   principalID,
		DayKey:        dayKey,
		IncludeParked: true,
		Limit:         2000,
	})
	if err != nil {
		return 0.5
	}

	maxSim := 0.0
	for _, s := range existing {
		cand := toTokenSet(strings.TrimSpace(s.Title) + " " + strings.TrimSpace(s.DraftSkill))
		sim := jaccard(tokens, cand)
		if sim > maxSim {
			maxSim = sim
		}
	}

	scarcity := 1.0 - maxSim
	if scarcity < 0 {
		return 0
	}
	if scarcity > 1 {
		return 1
	}
	return scarcity
}

func toTokenSet(s string) map[string]struct{} {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	parts := strings.Fields(b.String())
	out := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		if len(p) < 3 {
			continue
		}
		out[p] = struct{}{}
	}
	return out
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union <= 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

