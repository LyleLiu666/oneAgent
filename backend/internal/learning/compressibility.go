package learning

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

type CompressibilityVerdict string

const (
	CompressibilityVerdictEquivalent    CompressibilityVerdict = "equivalent"
	CompressibilityVerdictNotEquivalent CompressibilityVerdict = "not_equivalent"
	CompressibilityVerdictUncertain     CompressibilityVerdict = "uncertain"
)

type CompressibilityResult struct {
	CompressionPrompt string                 `json:"compression_prompt"`
	Verdict           CompressibilityVerdict `json:"verdict"`
	Reason            string                 `json:"reason"`
}

type CompressibilityEvaluator interface {
	Evaluate(ctx context.Context, principalID string, suggestion workledger.Suggestion) (CompressibilityResult, error)
}

// LLMCompressibilityEvaluator best-effort evaluates prompt compressibility.
// If no LLM model is configured, it returns ErrNoLLMConfigured.
type LLMCompressibilityEvaluator struct {
	Settings *settingsdb.DB

	mu      sync.Mutex
	inited  bool
	initErr error
	client  llm.Client
	model   string
	userID  string
}

var ErrNoLLMConfigured = errors.New("no llm configured")

func (e *LLMCompressibilityEvaluator) Evaluate(ctx context.Context, principalID string, suggestion workledger.Suggestion) (CompressibilityResult, error) {
	if e == nil || e.Settings == nil {
		return CompressibilityResult{}, ErrNoLLMConfigured
	}
	client, modelName, err := e.getClient(ctx, principalID)
	if err != nil {
		return CompressibilityResult{}, err
	}

	sys := `You are a strict evaluator for "Prompt Compressibility Test".
Return ONLY valid JSON. Do not include markdown fences.`

	user := fmt.Sprintf(`Given this SOP draft_skill, produce:
1) compression_prompt: a single short user prompt that tries to replace the SOP (simple question).
2) verdict: "equivalent" | "not_equivalent" | "uncertain"
3) reason: 1-3 sentences explaining why.

Criteria: If the compression_prompt is likely to get a similar end-to-end deliverable quality as the SOP, verdict="equivalent".
If the SOP contains workflow/edge cases/acceptance that a short prompt will miss, verdict="not_equivalent".

draft_skill:
%s
`, suggestion.DraftSkill)

	maxTokens := 600
	temp := 0.2
	out, err := client.ChatCompletion(ctx, []llm.ChatMessage{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	}, &llm.ChatCompletionOptions{
		Model:       modelName,
		MaxTokens:   &maxTokens,
		Temperature: &temp,
	})
	if err != nil {
		return CompressibilityResult{}, err
	}

	var parsed CompressibilityResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		return CompressibilityResult{}, fmt.Errorf("invalid json from llm: %w", err)
	}
	parsed.CompressionPrompt = strings.TrimSpace(parsed.CompressionPrompt)
	parsed.Reason = strings.TrimSpace(parsed.Reason)
	switch parsed.Verdict {
	case CompressibilityVerdictEquivalent, CompressibilityVerdictNotEquivalent, CompressibilityVerdictUncertain:
	default:
		parsed.Verdict = CompressibilityVerdictUncertain
	}
	return parsed, nil
}

func (e *LLMCompressibilityEvaluator) getClient(ctx context.Context, principalID string) (llm.Client, string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		principalID = "local"
	}

	if e.inited {
		if e.userID != principalID {
			// v1 is single-user; if a different principal shows up, re-init.
			e.inited = false
			e.client = nil
			e.model = ""
			e.initErr = nil
		} else {
			if e.initErr != nil {
				return nil, "", e.initErr
			}
			if e.client == nil || strings.TrimSpace(e.model) == "" {
				return nil, "", ErrNoLLMConfigured
			}
			return e.client, e.model, nil
		}
	}

	client, modelName, err := resolveLLMClient(ctx, e.Settings, principalID, "")
	if err != nil {
		// Treat "not configured" as non-fatal skip.
		if strings.Contains(err.Error(), "no LLM model configured") {
			e.inited = true
			e.userID = principalID
			e.client = nil
			e.model = ""
			e.initErr = ErrNoLLMConfigured
			return nil, "", ErrNoLLMConfigured
		}
		e.inited = true
		e.userID = principalID
		e.initErr = err
		return nil, "", err
	}
	e.inited = true
	e.userID = principalID
	e.client = client
	e.model = modelName
	e.initErr = nil
	return client, modelName, nil
}

func resolveLLMClient(ctx context.Context, settings *settingsdb.DB, userID, modelID string) (llm.Client, string, error) {
	if settings == nil {
		return nil, "", fmt.Errorf("settings db is required")
	}

	modelID = strings.TrimSpace(modelID)
	var m settingsdb.Model
	if modelID != "" {
		got, err := settings.GetModel(ctx, userID, modelID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, "", fmt.Errorf("model not found")
			}
			return nil, "", err
		}
		m = got
	} else {
		models, err := settings.ListModels(ctx, userID, "")
		if err != nil {
			return nil, "", err
		}
		for _, candidate := range models {
			if candidate.IsDefault {
				m = candidate
				break
			}
		}
		if m.ID == "" {
			return nil, "", fmt.Errorf("no LLM model configured")
		}
	}

	provider, err := settings.GetProvider(ctx, userID, m.ProviderID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", fmt.Errorf("provider not found")
		}
		return nil, "", err
	}

	if strings.TrimSpace(provider.BaseURL) == "" || strings.TrimSpace(provider.APIKey) == "" {
		return nil, "", fmt.Errorf("provider base_url or api_key is missing")
	}

	client, err := llm.NewClientForProvider(llm.ProviderConfig{
		ProviderType: provider.ProviderType,
		Endpoint:     provider.BaseURL,
		APIKey:       provider.APIKey,
		Model:        m.Model,
	})
	if err != nil {
		return nil, "", err
	}

	return client, m.Model, nil
}

func applyCompressibilityToScores(existing workledger.SuggestionScores, verdict CompressibilityVerdict) workledger.SuggestionScores {
	out := existing
	switch verdict {
	case CompressibilityVerdictEquivalent:
		// If a simple question likely works, it is shallow; cap depth.
		if out.DepthScore > 0.3 {
			out.DepthScore = 0.3
		}
	case CompressibilityVerdictNotEquivalent:
		// If it cannot be compressed, it's likely deeper; floor depth.
		if out.DepthScore < 0.7 {
			out.DepthScore = 0.7
		}
	default:
		// keep existing depth
	}
	out.TotalScore = 0.40*out.DepthScore + 0.35*out.ScarcityScore + 0.25*out.EvidenceScore
	return out
}

func shouldEvaluateSuggestion(s workledger.Suggestion) bool {
	// Only evaluate candidates that are still in governance scope.
	switch s.Status {
	case workledger.SuggestionStatusProposed, workledger.SuggestionStatusParked:
		return true
	default:
		return false
	}
}
