package secretary

import "testing"

func TestParseSecretaryTriagePlanTags_ToleratesMissingClosingContainerTags(t *testing.T) {
	// Intentionally omit </tasks>, </questions>, and </secretary_triage_plan> to simulate
	// typical truncated model output. Parser should still recover nested <task>/<item>.
	text := `
<secretary_triage_plan>
  <intent>dispatch</intent>
  <summary_message>ok</summary_message>
  <tasks>
    <task>
      <title>t1</title>
      <prompt>p1</prompt>
      <workspace_strategy>session</workspace_strategy>
    </task>
  <questions>
    <item>q1</item>
`

	plan, ok := parseSecretaryTriagePlanTags(text)
	if !ok {
		t.Fatalf("expected parse ok")
	}
	if plan.SummaryMessage != "ok" {
		t.Fatalf("expected summary_message=ok, got %q", plan.SummaryMessage)
	}
	if plan.Intent != "dispatch" {
		t.Fatalf("expected intent=dispatch, got %q", plan.Intent)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %+v", plan.Tasks)
	}
	if plan.Tasks[0].Title != "t1" || plan.Tasks[0].Prompt != "p1" || plan.Tasks[0].WorkspaceStrategy != "session" {
		t.Fatalf("unexpected task: %+v", plan.Tasks[0])
	}
	if len(plan.Questions) != 1 || plan.Questions[0] != "q1" {
		t.Fatalf("unexpected questions: %+v", plan.Questions)
	}
}
