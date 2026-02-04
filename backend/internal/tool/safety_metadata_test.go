package tool

import "testing"

func TestSafetyForToolID_PlanIsMutating(t *testing.T) {
	got := SafetyForToolID(ToolIDPlan)
	if got.Effect != SafetyEffectMutating {
		t.Fatalf("expected plan tool to be mutating, got %q", got.Effect)
	}
	if got.Reversibility != SafetyReversibilityRollbackable {
		t.Fatalf("expected plan tool to be rollbackable, got %q", got.Reversibility)
	}
}
