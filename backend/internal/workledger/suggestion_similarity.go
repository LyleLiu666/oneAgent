package workledger

import (
	"sort"
	"strings"
	"unicode"
)

type similarSuggestionV1 struct {
	SuggestionID string
	Similarity   float64
	Status       SuggestionStatus
}

func computeSimilarSuggestionsV1(targetTitle string, targetDraft string, candidates []Suggestion, limit int) []similarSuggestionV1 {
	if limit <= 0 {
		return nil
	}
	targetTokens := toTokenSetV1(strings.TrimSpace(targetTitle) + " " + strings.TrimSpace(targetDraft))
	if len(targetTokens) == 0 {
		return nil
	}

	out := make([]similarSuggestionV1, 0, limit)
	for _, s := range candidates {
		candTokens := toTokenSetV1(strings.TrimSpace(s.Title) + " " + strings.TrimSpace(s.DraftSkill))
		sim := jaccardV1(targetTokens, candTokens)
		if sim <= 0 {
			continue
		}
		out = append(out, similarSuggestionV1{
			SuggestionID: s.SuggestionID,
			Similarity:   sim,
			Status:       s.Status,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Similarity != out[j].Similarity {
			return out[i].Similarity > out[j].Similarity
		}
		return out[i].SuggestionID > out[j].SuggestionID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func toTokenSetV1(s string) map[string]struct{} {
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

func jaccardV1(a, b map[string]struct{}) float64 {
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

