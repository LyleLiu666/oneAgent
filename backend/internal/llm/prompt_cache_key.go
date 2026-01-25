package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type PromptCacheKeyInput struct {
	SessionID    string
	Epoch        int
	Model        string
	ToolProtocol string
	Messages     []ChatMessage
	Tools        []Tool
}

// BuildPromptCacheKey returns a stable cache key string suitable for providers that
// support `prompt_cache_key`.
//
// Key format:
//   v1:<session_id>:<epoch>:<stable_signature_hash>
//
// stable_signature_hash is derived from model + tool protocol + stable system messages
// + tool schema hash (for JSON tool calling).
func BuildPromptCacheKey(in PromptCacheKeyInput) (string, error) {
	sessionID := strings.TrimSpace(in.SessionID)
	if sessionID == "" {
		return "", fmt.Errorf("session_id is required")
	}

	epoch := in.Epoch
	if epoch < 0 {
		epoch = 0
	}

	model := strings.TrimSpace(in.Model)
	toolProtocol := strings.ToLower(strings.TrimSpace(in.ToolProtocol))

	systemParts := make([]string, 0, 2)
	for _, msg := range in.Messages {
		if msg.Volatile || msg.Role != "system" {
			continue
		}
		systemParts = append(systemParts, msg.Content)
	}
	stableSystem := strings.Join(systemParts, "\n\n")

	toolsHash := ""
	if len(in.Tools) > 0 && toolProtocol == "json" {
		ordered := normalizeTools(in.Tools)
		data, err := json.Marshal(ordered)
		if err != nil {
			return "", fmt.Errorf("marshal tools: %w", err)
		}
		sum := sha256.Sum256(data)
		toolsHash = hex.EncodeToString(sum[:])
	}

	signature := strings.Join([]string{
		"model=" + model,
		"tool_protocol=" + toolProtocol,
		"stable_system=" + stableSystem,
		"tools_hash=" + toolsHash,
	}, "\n")

	sum := sha256.Sum256([]byte(signature))
	sigHash := hex.EncodeToString(sum[:])

	return fmt.Sprintf("v1:%s:%d:%s", sessionID, epoch, sigHash), nil
}

