package permissions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"time"
)

type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

type Rule struct {
	ID          string      `json:"id"`
	Effect      Effect      `json:"effect"`
	ToolID      string      `json:"tool_id"`
	Constraints Constraints `json:"constraints,omitempty"`
}

type Constraints struct {
	FileScope            []string `json:"file_scope,omitempty"`
	ReadOutsideWorkspace *bool    `json:"read_outside_workspace,omitempty"`
	CommandProfile       string   `json:"command_profile,omitempty"`
	Allowlist            []string `json:"allowlist,omitempty"`
}

type Policy struct {
	ID                string `json:"id"`
	DefaultEffect     Effect `json:"default_effect,omitempty"`
	DefaultCmdProfile string `json:"default_command_profile,omitempty"`
	Rules             []Rule `json:"rules,omitempty"`
}

type Decision struct {
	Allowed     bool        `json:"allowed"`
	RuleID      string      `json:"rule_id,omitempty"`
	Reason      string      `json:"reason,omitempty"`
	Constraints Constraints `json:"constraints,omitempty"`
}

type Snapshot struct {
	Policy      Policy   `json:"policy"`
	PolicyHash  string   `json:"policy_hash"`
	ResolvedAt  string   `json:"resolved_at"`
	PrincipalID string   `json:"principal_id"`
}

func DefaultPolicy() Policy {
	return Policy{
		ID:                "default",
		DefaultEffect:     EffectAllow,
		DefaultCmdProfile: "dev",
		Rules: []Rule{
			{
				ID:     "bash-default",
				Effect: EffectAllow,
				ToolID: "bash",
				Constraints: Constraints{
					CommandProfile: "dev",
				},
			},
			{
				ID:     "run-command-default",
				Effect: EffectAllow,
				ToolID: "run_command",
				Constraints: Constraints{
					CommandProfile: "dev",
				},
			},
		},
	}
}

func ResolveSnapshot(principalID string, policy Policy, now time.Time) Snapshot {
	if strings.TrimSpace(principalID) == "" {
		principalID = "local"
	}
	if policy.ID == "" {
		policy.ID = "default"
	}
	hash := HashPolicy(policy)
	return Snapshot{
		Policy:      policy,
		PolicyHash:  hash,
		ResolvedAt:  now.UTC().Format(time.RFC3339),
		PrincipalID: principalID,
	}
}

func HashPolicy(policy Policy) string {
	data, _ := json.Marshal(policy)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func Evaluate(policy Policy, toolID string) Decision {
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		return Decision{Allowed: false, Reason: "missing_tool_id"}
	}
	if isToolDisabledByEnv(toolID) {
		return Decision{Allowed: false, Reason: "disabled_by_env"}
	}

	// Deny rules take precedence.
	for _, rule := range policy.Rules {
		if !matchesTool(rule.ToolID, toolID) {
			continue
		}
		if rule.Effect == EffectDeny {
			return Decision{
				Allowed: false,
				RuleID:  rule.ID,
				Reason:  "denied_by_rule",
			}
		}
	}

	// Allow rules.
	for _, rule := range policy.Rules {
		if !matchesTool(rule.ToolID, toolID) {
			continue
		}
		if rule.Effect == EffectAllow {
			return Decision{
				Allowed:     true,
				RuleID:      rule.ID,
				Reason:      "allowed_by_rule",
				Constraints: rule.Constraints,
			}
		}
	}

	if policy.DefaultEffect == EffectDeny {
		return Decision{Allowed: false, Reason: "default_deny"}
	}
	return Decision{
		Allowed: true,
		Reason:  "default_allow",
		Constraints: Constraints{
			CommandProfile: policy.DefaultCmdProfile,
		},
	}
}

func matchesTool(ruleToolID, toolID string) bool {
	ruleToolID = strings.TrimSpace(ruleToolID)
	if ruleToolID == "" || ruleToolID == "*" {
		return true
	}
	return ruleToolID == toolID
}

func toolDisableEnvVar(id string) string {
	return "ONEAGENT_DISABLE_TOOL_" + strings.ToUpper(id)
}

func isToolDisabledByEnv(id string) bool {
	v := strings.TrimSpace(os.Getenv(toolDisableEnvVar(id)))
	return v == "1" || strings.EqualFold(v, "true")
}

