package permissions

import "testing"

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

