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
	if !strings.Contains(prompt, "questions[]") {
		t.Fatalf("expected SW prompt to mention questions[] guidance, got %q", prompt)
	}
	if !strings.Contains(prompt, "硬阻塞") && !strings.Contains(prompt, "硬阻断") {
		t.Fatalf("expected SW prompt to describe questions[] as hard blockers, got %q", prompt)
	}
	if !strings.Contains(prompt, "最多 1") && !strings.Contains(prompt, "最多1") {
		t.Fatalf("expected SW prompt to limit questions per round, got %q", prompt)
	}
}

