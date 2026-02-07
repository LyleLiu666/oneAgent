package secretary

import (
	"context"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/sessionstore"
	"github.com/liu_y/oneAgent/backend/internal/toolcalling"
)

func TestGenerateTriagePlanAsSU_ToolLoopBudget_IsDecisionOnly_NotHardLimited(t *testing.T) {
	sessions, err := sessionstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("new sessionstore: %v", err)
	}

	client := &scriptedStreamClient{
		responses: []string{
			`<tool_data><call><tool_name>does_not_exist</tool_name></call></tool_data>`,
			`<tool_data><call><tool_name>does_not_exist</tool_name></call></tool_data>`,
			`<tool_data><call><tool_name>does_not_exist</tool_name></call></tool_data>`,
			`<tool_data><call><tool_name>does_not_exist</tool_name></call></tool_data>`,
			`<tool_data><call><tool_name>does_not_exist</tool_name></call></tool_data>`,
			`<secretary_triage_plan>
  <intent>progress</intent>
  <summary_message>ok</summary_message>
  <tasks></tasks>
  <task_actions></task_actions>
  <questions></questions>
</secretary_triage_plan>`,
		},
	}

	o := &Orchestrator{
		Sessions: sessions,
		ResolveModel: func(ctx context.Context, userID, modelID string) (llm.Client, string, error) {
			return client, "mock", nil
		},
	}

	ctx := toolcalling.ContextWithChatToolMaxSteps(context.Background(), 6)
	plan, _, err := o.generateTriagePlanAsSU(ctx, "local", "session-1", "", "", model.JSONB{}, []model.ChatMessage{
		{Role: model.MessageRoleUser, Type: model.MessageTypeText, Content: "do it"},
	}, triageCarryContext{})
	if err != nil {
		t.Fatalf("generateTriagePlanAsSU: %v", err)
	}
	if strings.TrimSpace(plan.SummaryMessage) != "ok" {
		t.Fatalf("expected plan summary ok, got %q", plan.SummaryMessage)
	}
	if client.callCount != 6 {
		t.Fatalf("expected 6 streaming calls (tool-loop steps), got %d", client.callCount)
	}
}
