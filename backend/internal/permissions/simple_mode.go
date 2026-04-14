package permissions

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/shell"
)

type SimpleMode string

const (
	SimpleModeReadonly      SimpleMode = "readonly"
	SimpleModeSandboxCoding SimpleMode = "sandbox_coding"
	SimpleModeHostFull      SimpleMode = "host_full"
	SimpleModeCustom        SimpleMode = "custom"
)

type SimpleModeInfo struct {
	Mode        SimpleMode `json:"mode"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	RiskLevel   string     `json:"risk_level"`
}

type SimpleModeUnavailableError struct {
	Mode   SimpleMode
	Reason string
}

func (e *SimpleModeUnavailableError) Error() string {
	if e == nil {
		return "simple mode unavailable"
	}
	if strings.TrimSpace(e.Reason) != "" {
		return strings.TrimSpace(e.Reason)
	}
	return fmt.Sprintf("simple mode %s is unavailable", e.Mode)
}

type sandboxSupport struct {
	native bool
	docker bool
}

var detectSandboxSupport = defaultSandboxSupport
var sandboxLookPath = exec.LookPath

// Keep these lists aligned with backend/internal/tool/safety_metadata.go.
var simpleModeReadOnlyToolIDs = []string{
	"read_file",
	"ls",
	"glob",
	"rg",
	"search",
	"skill_read",
	"lsp_definition",
	"lsp_references",
	"lsp_rename_preview",
}

// Command tools are classified separately because they also carry profile/sandbox semantics.
var simpleModeNonCommandMutatingToolIDs = []string{
	"write_file",
	"edit",
	"edit_v2",
	"multiedit",
	"trash_file",
	"document_export",
	"plan",
	"subagent",
}

func defaultSandboxSupport() sandboxSupport {
	support := sandboxSupport{}
	if runtime.GOOS == "darwin" {
		if _, err := sandboxLookPath("sandbox-exec"); err == nil {
			support.native = true
		}
	}
	if _, err := sandboxLookPath("docker"); err == nil {
		support.docker = true
	}
	return support
}

func SimpleModes() []SimpleMode {
	return []SimpleMode{
		SimpleModeReadonly,
		SimpleModeSandboxCoding,
		SimpleModeHostFull,
		SimpleModeCustom,
	}
}

func PresetSimpleModes() []SimpleMode {
	return []SimpleMode{
		SimpleModeReadonly,
		SimpleModeSandboxCoding,
		SimpleModeHostFull,
	}
}

func SimpleModeDetails(mode SimpleMode) SimpleModeInfo {
	switch mode {
	case SimpleModeReadonly:
		return SimpleModeInfo{
			Mode:        mode,
			Label:       "只读查看",
			Description: "可读、可查、不可改。",
			RiskLevel:   "low",
		}
	case SimpleModeSandboxCoding:
		return SimpleModeInfo{
			Mode:        mode,
			Label:       "沙箱开发",
			Description: "允许改代码和跑测试，但尽量放在隔离环境里。",
			RiskLevel:   "medium",
		}
	case SimpleModeHostFull:
		return SimpleModeInfo{
			Mode:        mode,
			Label:       "本机执行",
			Description: "直接在当前机器执行，能力最强，风险最高。",
			RiskLevel:   "high",
		}
	default:
		return SimpleModeInfo{
			Mode:        SimpleModeCustom,
			Label:       "自定义",
			Description: "当前策略不是系统预设，需要高级设置查看。",
			RiskLevel:   "custom",
		}
	}
}

func DetectPreferredHardBoundarySandbox() (shell.SandboxMode, bool, string) {
	support := detectSandboxSupport()
	switch {
	case support.native:
		return shell.SandboxModeNative, true, ""
	case support.docker:
		return shell.SandboxModeDocker, true, ""
	default:
		return shell.SandboxModeNone, false, "当前机器没有可用的隔离执行环境；可改用本机执行，或先安装 / 启用 Docker"
	}
}

func DefaultCommandToolSandboxMode() shell.SandboxMode {
	if mode, ok, _ := DetectPreferredHardBoundarySandbox(); ok && mode == shell.SandboxModeNative {
		return mode
	}
	return shell.SandboxModeNone
}

func BuildSimpleModePolicy(mode SimpleMode) (Policy, error) {
	switch mode {
	case SimpleModeReadonly:
		return buildReadonlySimpleModePolicy(), nil
	case SimpleModeHostFull:
		return buildCommandProfilePolicy("simple_host_full", "full", string(shell.SandboxModeHost)), nil
	case SimpleModeSandboxCoding:
		sandboxMode, ok, reason := DetectPreferredHardBoundarySandbox()
		if !ok {
			return Policy{}, &SimpleModeUnavailableError{
				Mode:   mode,
				Reason: reason,
			}
		}
		return buildCommandProfilePolicy("simple_sandbox_coding", "coding", string(sandboxMode)), nil
	default:
		return Policy{}, fmt.Errorf("unsupported simple mode %q", mode)
	}
}

func ClassifySimpleMode(policy Policy) SimpleMode {
	bashSemantics, ok := commandSemanticsForPolicy(policy, "bash")
	if !ok {
		return SimpleModeCustom
	}
	runSemantics, ok := commandSemanticsForPolicy(policy, "run_command")
	if !ok {
		return SimpleModeCustom
	}
	if bashSemantics != runSemantics {
		return SimpleModeCustom
	}
	if !allSimpleModeToolsAllowed(policy, simpleModeReadOnlyToolIDs) {
		return SimpleModeCustom
	}

	switch {
	case bashSemantics.rawProfile == "full" &&
		bashSemantics.sandboxMode == shell.SandboxModeHost &&
		allSimpleModeToolsAllowed(policy, simpleModeNonCommandMutatingToolIDs):
		return SimpleModeHostFull
	case bashSemantics.rawProfile == "coding" &&
		bashSemantics.sandboxMode.IsHardBoundary() &&
		allSimpleModeToolsAllowed(policy, simpleModeNonCommandMutatingToolIDs):
		return SimpleModeSandboxCoding
	case (bashSemantics.rawProfile == "readonly" || bashSemantics.effectiveProfile == "readonly") &&
		allSimpleModeToolsDenied(policy, simpleModeNonCommandMutatingToolIDs):
		return SimpleModeReadonly
	default:
		return SimpleModeCustom
	}
}

type commandSemantics struct {
	rawProfile       string
	effectiveProfile string
	sandboxMode      shell.SandboxMode
}

func commandSemanticsForPolicy(policy Policy, toolID string) (commandSemantics, bool) {
	decision := Evaluate(policy, toolID)
	if !decision.Allowed {
		return commandSemantics{}, false
	}

	profile := strings.ToLower(strings.TrimSpace(decision.Constraints.CommandProfile))
	if profile == "" {
		profile = "dev"
	}

	mode, ok := resolveConstraintSandboxMode(decision.Constraints.SandboxMode)
	if !ok {
		return commandSemantics{}, false
	}
	if profile == "coding" && !mode.IsHardBoundary() {
		return commandSemantics{}, false
	}

	effective := profile
	if mode == shell.SandboxModeNone {
		effective = "readonly"
	}
	return commandSemantics{
		rawProfile:       profile,
		effectiveProfile: effective,
		sandboxMode:      mode,
	}, true
}

func resolveConstraintSandboxMode(raw string) (shell.SandboxMode, bool) {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		mode, err := shell.ParseSandboxMode(raw)
		if err != nil {
			return "", false
		}
		return mode, true
	}
	return DefaultCommandToolSandboxMode(), true
}

func buildReadonlySimpleModePolicy() Policy {
	rules := []Rule{
		commandProfileRule("bash-default", "bash", "readonly", ""),
		commandProfileRule("run-command-default", "run_command", "readonly", ""),
	}
	for _, toolID := range simpleModeReadOnlyToolIDs {
		rules = append(rules, allowToolRule("allow-"+toolID, toolID))
	}
	return Policy{
		ID:                "simple_readonly",
		DefaultEffect:     EffectDeny,
		DefaultCmdProfile: "readonly",
		Rules:             rules,
	}
}

func buildCommandProfilePolicy(id, profile, sandboxMode string) Policy {
	profile = strings.ToLower(strings.TrimSpace(profile))
	sandboxMode = strings.ToLower(strings.TrimSpace(sandboxMode))

	return Policy{
		ID:                id,
		DefaultEffect:     EffectAllow,
		DefaultCmdProfile: profile,
		Rules: []Rule{
			commandProfileRule("bash-default", "bash", profile, sandboxMode),
			commandProfileRule("run-command-default", "run_command", profile, sandboxMode),
		},
	}
}

func allowToolRule(id, toolID string) Rule {
	return Rule{
		ID:     id,
		Effect: EffectAllow,
		ToolID: toolID,
	}
}

func commandProfileRule(id, toolID, profile, sandboxMode string) Rule {
	constraints := Constraints{
		CommandProfile: profile,
		Approval:       "high_risk",
	}
	if sandboxMode != "" {
		constraints.SandboxMode = sandboxMode
	}
	return Rule{
		ID:          id,
		Effect:      EffectAllow,
		ToolID:      toolID,
		Constraints: constraints,
	}
}

func allSimpleModeToolsAllowed(policy Policy, toolIDs []string) bool {
	for _, toolID := range toolIDs {
		if !Evaluate(policy, toolID).Allowed {
			return false
		}
	}
	return true
}

func allSimpleModeToolsDenied(policy Policy, toolIDs []string) bool {
	for _, toolID := range toolIDs {
		if Evaluate(policy, toolID).Allowed {
			return false
		}
	}
	return true
}
