package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	agentsdkprovider "codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/agentsdk.git/provider"
)

type PromptCacheKeyInput struct {
	SessionID    string
	Epoch        int
	Model        string
	ToolProtocol string
	Messages     []ChatMessage
	Tools        []Tool
}

func BuildPromptCacheKey(in PromptCacheKeyInput) (string, error) {
	providerMessages := make([]agentsdkprovider.ChatMessage, 0, len(in.Messages))
	for _, msg := range in.Messages {
		providerMessages = append(providerMessages, agentsdkprovider.ChatMessage{
			Role:                msg.Role,
			Content:             msg.Content,
			PromptCacheBehavior: toSDKPromptCacheBehavior(msg),
		})
	}

	providerTools := make([]agentsdkprovider.ToolSpec, 0, len(in.Tools))
	for _, tool := range normalizeTools(in.Tools) {
		if tool.Type != "" && tool.Type != "function" {
			continue
		}
		params, err := json.Marshal(tool.Function.Parameters)
		if err != nil {
			return "", err
		}
		providerTools = append(providerTools, agentsdkprovider.ToolSpec{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: string(params),
		})
	}

	key, err := agentsdkprovider.BuildPromptCacheKey(agentsdkprovider.PromptCacheKeyInput{
		SessionID:     in.SessionID,
		PrefixVersion: in.Epoch,
		Model:         in.Model,
		ToolProtocol:  in.ToolProtocol,
		Messages:      providerMessages,
		Tools:         providerTools,
	})
	if err != nil {
		return "", err
	}
	return normalizePromptCacheKeyLength(key), nil
}

func normalizePromptCacheKeyLength(key string) string {
	key = strings.TrimSpace(key)
	if key == "" || len(key) <= 64 {
		return key
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
