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

func TestParseSecretaryTriagePlanTags_ParsesTaskActions(t *testing.T) {
	text := `
<secretary_triage_plan>
  <intent>dispatch</intent>
  <summary_message>ok</summary_message>
  <tasks></tasks>
  <task_actions>
    <task_action>
      <action>cancel</action>
      <task_id>abcd1234</task_id>
    </task_action>
    <task_action>
      <action>resume</action>
      <task_id>efgh5678</task_id>
      <review_notes>continue</review_notes>
    </task_action>
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
	if len(plan.TaskActions) != 2 {
		t.Fatalf("expected 2 task_actions, got %+v", plan.TaskActions)
	}
	if plan.TaskActions[0].Action != "cancel" || plan.TaskActions[0].TaskID != "abcd1234" {
		t.Fatalf("unexpected first action: %+v", plan.TaskActions[0])
	}
	if plan.TaskActions[1].Action != "resume" || plan.TaskActions[1].TaskID != "efgh5678" || plan.TaskActions[1].ReviewNotes != "continue" {
		t.Fatalf("unexpected second action: %+v", plan.TaskActions[1])
	}
	if len(plan.Questions) != 1 || plan.Questions[0] != "q1" {
		t.Fatalf("unexpected questions: %+v", plan.Questions)
	}
}
