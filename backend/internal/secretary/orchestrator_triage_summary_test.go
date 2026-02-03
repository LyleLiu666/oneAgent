package secretary

import (
	"strings"
	"testing"
)

func TestBuildTriageSummary_QuestionsAreVisible(t *testing.T) {
	out := buildTriageSummary("", 0, []string{"请告诉我用哪个目录来执行。"})
	if !strings.Contains(out, "请告诉我用哪个目录来执行。") {
		t.Fatalf("expected question to be included, got %q", out)
	}
	if strings.Contains(out, "有 1 个问题") || strings.Contains(out, "有1个问题") {
		t.Fatalf("expected no count-only phrasing, got %q", out)
	}
}

func TestBuildTriageSummary_AppendsQuestionsToPlanSummary(t *testing.T) {
	out := buildTriageSummary("我在推进中。", 1, []string{"需要你确认：用哪个目录？"})
	if !strings.Contains(out, "我在推进中。") {
		t.Fatalf("expected plan summary to be preserved, got %q", out)
	}
	if !strings.Contains(out, "需要你确认：用哪个目录？") {
		t.Fatalf("expected question to be appended, got %q", out)
	}
}

func TestBuildTriageSummary_DoesNotDuplicateExistingQuestion(t *testing.T) {
	out := buildTriageSummary("继续之前我需要你拍板：\n1) 用哪个目录？", 0, []string{"用哪个目录？"})
	if strings.Count(out, "用哪个目录？") != 1 {
		t.Fatalf("expected question to appear once, got %q", out)
	}
}
