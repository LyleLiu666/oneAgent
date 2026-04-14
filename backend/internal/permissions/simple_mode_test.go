package permissions

import (
	"errors"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/shell"
)

func TestBuildSimpleModePolicy_SandboxCodingRequiresHardBoundary(t *testing.T) {
	original := detectSandboxSupport
	detectSandboxSupport = func() sandboxSupport { return sandboxSupport{} }
	t.Cleanup(func() { detectSandboxSupport = original })

	_, err := BuildSimpleModePolicy(SimpleModeSandboxCoding)
	if err == nil {
		t.Fatalf("expected sandbox coding to fail without a hard boundary")
	}
	var unavailable *SimpleModeUnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("expected SimpleModeUnavailableError, got %T", err)
	}
}

func TestBuildSimpleModePolicy_ReadonlyDeniesMutatingTools(t *testing.T) {
	policy, err := BuildSimpleModePolicy(SimpleModeReadonly)
	if err != nil {
		t.Fatalf("build readonly policy: %v", err)
	}

	if dec := Evaluate(policy, "read_file"); !dec.Allowed {
		t.Fatalf("expected read_file to remain allowed, got %+v", dec)
	}
	if dec := Evaluate(policy, "write_file"); dec.Allowed {
		t.Fatalf("expected write_file to be denied, got %+v", dec)
	}
	if dec := Evaluate(policy, "edit"); dec.Allowed {
		t.Fatalf("expected edit to be denied, got %+v", dec)
	}
	if dec := Evaluate(policy, "trash_file"); dec.Allowed {
		t.Fatalf("expected trash_file to be denied, got %+v", dec)
	}
}

func TestClassifySimpleMode_Presets(t *testing.T) {
	tests := []struct {
		name    string
		mode    SimpleMode
		support sandboxSupport
		want    SimpleMode
	}{
		{
			name: "readonly",
			mode: SimpleModeReadonly,
			want: SimpleModeReadonly,
		},
		{
			name: "host full",
			mode: SimpleModeHostFull,
			want: SimpleModeHostFull,
		},
		{
			name: "sandbox coding with native",
			mode: SimpleModeSandboxCoding,
			support: sandboxSupport{
				native: true,
			},
			want: SimpleModeSandboxCoding,
		},
		{
			name: "sandbox coding with docker",
			mode: SimpleModeSandboxCoding,
			support: sandboxSupport{
				docker: true,
			},
			want: SimpleModeSandboxCoding,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := detectSandboxSupport
			detectSandboxSupport = func() sandboxSupport { return tt.support }
			t.Cleanup(func() { detectSandboxSupport = original })

			policy, err := BuildSimpleModePolicy(tt.mode)
			if err != nil {
				t.Fatalf("build policy: %v", err)
			}
			if got := ClassifySimpleMode(policy); got != tt.want {
				t.Fatalf("expected mode=%s, got %s", tt.want, got)
			}
		})
	}
}

func TestClassifySimpleMode_DefaultPolicyFollowsEffectiveBehavior(t *testing.T) {
	original := detectSandboxSupport
	t.Cleanup(func() { detectSandboxSupport = original })

	detectSandboxSupport = func() sandboxSupport { return sandboxSupport{} }
	if got := ClassifySimpleMode(DefaultPolicy()); got != SimpleModeCustom {
		t.Fatalf("expected default policy without hard boundary to classify as custom, got %s", got)
	}

	detectSandboxSupport = func() sandboxSupport { return sandboxSupport{native: true} }
	if DefaultCommandToolSandboxMode() != shell.SandboxModeNative {
		t.Fatalf("expected native sandbox default when native support is available")
	}
	if got := ClassifySimpleMode(DefaultPolicy()); got != SimpleModeSandboxCoding {
		t.Fatalf("expected default policy with native sandbox to classify as sandbox_coding, got %s", got)
	}
}

func TestClassifySimpleMode_CustomWhenCommandPoliciesDiverge(t *testing.T) {
	policy := Policy{
		ID:                "custom",
		DefaultEffect:     EffectAllow,
		DefaultCmdProfile: "readonly",
		Rules: []Rule{
			commandProfileRule("bash-default", "bash", "full", string(shell.SandboxModeHost)),
		},
	}

	if got := ClassifySimpleMode(policy); got != SimpleModeCustom {
		t.Fatalf("expected custom classification, got %s", got)
	}
}

func TestClassifySimpleMode_CustomWhenMutatingToolSemanticsDiverge(t *testing.T) {
	policy, err := BuildSimpleModePolicy(SimpleModeHostFull)
	if err != nil {
		t.Fatalf("build host_full policy: %v", err)
	}
	policy.Rules = append(policy.Rules, Rule{
		ID:     "deny-write-file",
		Effect: EffectDeny,
		ToolID: "write_file",
	})

	if got := ClassifySimpleMode(policy); got != SimpleModeCustom {
		t.Fatalf("expected custom classification, got %s", got)
	}
}
