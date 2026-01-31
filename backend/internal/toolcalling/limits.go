package toolcalling

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultChatToolMaxSteps = 200
	maxChatToolMaxStepsCap  = 2000
)

// ChatToolMaxSteps returns the max number of tool-loop iterations allowed for chat.
//
// This limit applies to both JSON-native tool calling and XML tool_data fallback loops.
//
// Env overrides:
// - ONEAGENT_CHAT_TOOL_MAX_STEPS
// - ONEAGENT_CHAT_TOOL_MAX_STEPS_CAP
func ChatToolMaxSteps() int {
	maxSteps := defaultChatToolMaxSteps
	if v := envInt("ONEAGENT_CHAT_TOOL_MAX_STEPS"); v > 0 {
		maxSteps = v
	}
	if v := envInt("ONEAGENT_CHAT_TOOL_MAX_STEPS_CAP"); v > 0 && maxSteps > v {
		maxSteps = v
	}
	if maxSteps < 1 {
		return 1
	}
	if maxSteps > maxChatToolMaxStepsCap {
		return maxChatToolMaxStepsCap
	}
	return maxSteps
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
