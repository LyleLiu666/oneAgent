package permissions

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestEvaluate_DenyOverridesAllow(t *testing.T) {
	policy := Policy{
		ID:            "test",
		DefaultEffect: EffectAllow,
		Rules: []Rule{
			{ID: "allow-bash", Effect: EffectAllow, ToolID: "bash"},
			{ID: "deny-bash", Effect: EffectDeny, ToolID: "bash"},
		},
	}

	dec := Evaluate(policy, "bash")
	if dec.Allowed {
		t.Fatalf("expected deny, got allow")
	}
	if dec.RuleID != "deny-bash" {
		t.Fatalf("expected deny-bash, got %s", dec.RuleID)
	}
}

func TestEvaluate_DefaultAllow(t *testing.T) {
	policy := DefaultPolicy()
	dec := Evaluate(policy, "read_file")
	if !dec.Allowed {
		t.Fatalf("expected allow")
	}
	if dec.Reason != "default_allow" && dec.Reason != "allowed_by_rule" {
		t.Fatalf("unexpected reason: %s", dec.Reason)
	}
}

func TestDefaultPolicy_CommandTools_UsesCodingOnDarwinWhenNativeSandboxAvailable(t *testing.T) {
	want := "dev"
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("sandbox-exec"); err == nil {
			want = "coding"
		}
	}

	policy := DefaultPolicy()
	var gotBash, gotRun string
	for _, rule := range policy.Rules {
		switch rule.ToolID {
		case "bash":
			gotBash = rule.Constraints.CommandProfile
		case "run_command":
			gotRun = rule.Constraints.CommandProfile
		}
	}

	if gotBash != want {
		t.Fatalf("bash command_profile=%q want=%q", gotBash, want)
	}
	if gotRun != want {
		t.Fatalf("run_command command_profile=%q want=%q", gotRun, want)
	}
}
