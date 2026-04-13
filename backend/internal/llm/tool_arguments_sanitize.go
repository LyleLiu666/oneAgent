package llm

import agentsdk "codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/agentsdk.git"

// SanitizeToolArgumentsJSON guarantees a non-empty, valid JSON string for provider tool arguments.
// If the input cannot be parsed as JSON, it is wrapped as `{"_raw": "<original>"}`.
func SanitizeToolArgumentsJSON(args string) string {
	return agentsdk.SanitizeToolArgumentsJSON(args)
}
