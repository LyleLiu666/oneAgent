package secretary

import (
	"context"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func TestDispatchPlanAsSW_DoesNotInjectWorkspaceQuestion_WhenPlanAlreadyHasQuestions(t *testing.T) {
	o := &Orchestrator{
		Tasks:  &taskqueue.Store{},
		Runner: &taskqueue.TaskRunner{},
	}

	plan := triagePlan{
		Tasks: []triageTask{{
			Title:             "跑测试",
			Prompt:            "run tests",
			WorkspaceStrategy: "session",
		}},
		Questions: []string{"请把仓库根目录路径发我，我才能继续。"},
	}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", "", plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}

	if len(got.CreatedTaskIDs) != 0 {
		t.Fatalf("expected no dispatched tasks, got %v", got.CreatedTaskIDs)
	}
	if len(got.Questions) != 1 {
		t.Fatalf("expected only plan question, got %v", got.Questions)
	}
	if got.Questions[0] != plan.Questions[0] {
		t.Fatalf("expected question %q, got %q", plan.Questions[0], got.Questions[0])
	}
}

func TestDispatchPlanAsSW_AddsFallbackQuestion_WhenWorkspaceIsRequiredButNoQuestionsProvided(t *testing.T) {
	o := &Orchestrator{
		Tasks:  &taskqueue.Store{},
		Runner: &taskqueue.TaskRunner{},
	}

	plan := triagePlan{
		Tasks: []triageTask{{
			Title:             "跑测试",
			Prompt:            "run tests",
			WorkspaceStrategy: "session",
		}},
	}

	got, err := o.dispatchPlanAsSW(context.Background(), "local", "s", "", plan)
	if err != nil {
		t.Fatalf("dispatchPlanAsSW: %v", err)
	}

	if len(got.CreatedTaskIDs) != 0 {
		t.Fatalf("expected no dispatched tasks, got %v", got.CreatedTaskIDs)
	}
	if len(got.Questions) == 0 {
		t.Fatalf("expected a fallback question, got none")
	}
}
