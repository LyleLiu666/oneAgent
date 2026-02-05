package secretary

import (
	"strings"
	"testing"
)

func TestSecretaryReportSystemPromptSU_PrefersDefaultsOverOptionMenus(t *testing.T) {
	prompt := strings.TrimSpace(secretaryReportSystemPromptSU)
	if !strings.Contains(prompt, "ONEAGENT_SECRETARY_SU_REPORT") {
		t.Fatalf("expected marker in SU prompt, got %q", prompt)
	}
	if strings.Contains(prompt, "2~3 个可选项") || strings.Contains(prompt, "2-3 个可选项") {
		t.Fatalf("expected SU prompt to avoid forcing option menus, got %q", prompt)
	}
	if !strings.Contains(prompt, "默认") {
		t.Fatalf("expected SU prompt to encourage default decisions, got %q", prompt)
	}
}

func TestSecretaryDispatchSystemPromptSW_QuestionsAreHardBlockersAndLimited(t *testing.T) {
	prompt := strings.TrimSpace(secretaryDispatchSystemPromptSW)
	if !strings.Contains(prompt, "ONEAGENT_SECRETARY_TRIAGE") {
		t.Fatalf("expected marker in SW prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "secretary_triage_plan") {
		t.Fatalf("expected SW prompt to mention secretary_triage_plan protocol, got %q", prompt)
	}
	if !strings.Contains(prompt, "<secretary_triage_plan>") {
		t.Fatalf("expected SW prompt to mention <secretary_triage_plan> tags fallback, got %q", prompt)
	}
	if !strings.Contains(prompt, "5") || !strings.Contains(prompt, "工具调用") {
		t.Fatalf("expected SW prompt to mention tool-call budget, got %q", prompt)
	}
	if !strings.Contains(prompt, "questions[]") {
		t.Fatalf("expected SW prompt to mention questions[] guidance, got %q", prompt)
	}
	if !strings.Contains(prompt, "硬阻塞") && !strings.Contains(prompt, "硬阻断") {
		t.Fatalf("expected SW prompt to describe questions[] as hard blockers, got %q", prompt)
	}
	if !strings.Contains(prompt, "每轮最多问 1") && !strings.Contains(prompt, "每轮最多问1") {
		t.Fatalf("expected SW prompt to limit questions per turn, got %q", prompt)
	}
	if !strings.Contains(prompt, "严禁编造进度") {
		t.Fatalf("expected SW prompt to include anti-hallucination guardrail, got %q", prompt)
	}
	if !strings.Contains(prompt, "task_actions") {
		t.Fatalf("expected SW prompt to mention task_actions, got %q", prompt)
	}
	if !strings.Contains(prompt, "不允许 delete") && !strings.Contains(prompt, "绝不 delete") {
		t.Fatalf("expected SW prompt to disallow delete, got %q", prompt)
	}
}
