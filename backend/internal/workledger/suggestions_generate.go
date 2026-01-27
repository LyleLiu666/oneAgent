package workledger

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

type GenerateSuggestionsInput struct {
	PrincipalID string
	DayKey      string

	LookbackDays int
	Count        int
}

// GenerateSuggestionsV1 is a best-effort suggestion generator.
// It is safe to run repeatedly: suggestions are deduped by evidence signature within the day.
func (s *Store) GenerateSuggestionsV1(ctx context.Context, in GenerateSuggestionsInput) ([]Suggestion, error) {
	if s == nil {
		return nil, errors.New("store is nil")
	}
	principal := strings.TrimSpace(in.PrincipalID)
	if principal == "" {
		return nil, errors.New("principal_id is required")
	}

	dayKey := strings.TrimSpace(in.DayKey)
	if dayKey == "" {
		dayKey = DayKey(time.Now())
	}

	lookback := in.LookbackDays
	if lookback <= 0 {
		lookback = 7
	}
	if lookback > 30 {
		lookback = 30
	}
	count := in.Count
	if count <= 0 {
		count = 1
	}
	if count > 3 {
		count = 3
	}

	after := time.Now().Add(-time.Duration(lookback) * 24 * time.Hour).UTC()
	receipts, err := s.ListReceipts(ListReceiptsQuery{
		PrincipalID:   principal,
		Status:        ReceiptStatusSucceeded,
		FinishedAfter: after,
		Limit:         500,
	})
	if err != nil {
		return nil, err
	}

	usable := make([]Receipt, 0, len(receipts))
	for _, r := range receipts {
		if strings.TrimSpace(r.Artifacts.FindingsPath) == "" || strings.TrimSpace(r.Artifacts.TraceLogPath) == "" {
			continue
		}
		usable = append(usable, r)
	}
	if len(usable) < 2 {
		return []Suggestion{}, nil
	}

	sort.SliceStable(usable, func(i, j int) bool { return usable[i].FinishedAt.After(usable[j].FinishedAt) })

	existingSigs := s.loadExistingEvidenceSignatures(principal, dayKey)
	created := make([]Suggestion, 0, count)
	usedReceiptIDs := map[string]bool{}

	for len(created) < count {
		select {
		case <-ctx.Done():
			return created, ctx.Err()
		default:
		}

		pair := pickNextPairV1(usable, usedReceiptIDs)
		if len(pair) < 2 {
			break
		}
		for _, r := range pair {
			usedReceiptIDs[r.ReceiptID] = true
		}

		evidence := []string{pair[0].ReceiptID, pair[1].ReceiptID}
		sig := evidenceSignatureV1(evidence)
		if existingSigs[sig] {
			continue
		}
		title := deriveSuggestionTitleV1(pair)
		draft := buildDraftSkillV1(pair)
		scores := ComputeSuggestionScores(s, principal, dayKey, title, draft, evidence)

		sug, err := s.CreateSuggestion(CreateSuggestionInput{
			PrincipalID:        principal,
			WorkspaceRoot:      strings.TrimSpace(pair[0].WorkspaceRoot),
			Title:              title,
			Description:        "Auto-proposed from recent successful receipts (manual review required).",
			RiskNotes:          "This is a proposed SOP. Review carefully before approving.",
			EvidenceReceiptIDs: evidence,
			DraftSkill:         draft,
			Scores:             scores,
			Meta:               SuggestionMeta{DayKey: dayKey},
		})
		if err != nil {
			return created, err
		}

		existingSigs[sig] = true
		_ = s.ApplyInboxCap(principal, dayKey, 10)
		created = append(created, sug)
	}

	return created, nil
}

func pickNextPairV1(receipts []Receipt, used map[string]bool) []Receipt {
	out := make([]Receipt, 0, 2)
	for _, r := range receipts {
		if used[r.ReceiptID] {
			continue
		}
		out = append(out, r)
		if len(out) == 2 {
			return out
		}
	}
	return nil
}

func deriveSuggestionTitleV1(rs []Receipt) string {
	if len(rs) == 0 {
		return "SOP"
	}
	s := strings.TrimSpace(rs[0].Summary)
	if s == "" {
		s = "recent work"
	}
	s = strings.ReplaceAll(s, "\n", " ")
	if len([]rune(s)) > 60 {
		s = string([]rune(s)[:60]) + "…"
	}
	return "SOP: " + s
}

func buildDraftSkillV1(rs []Receipt) string {
	var b strings.Builder
	b.WriteString("# SOP (Proposed)\n\n")
	b.WriteString("## 使用时机 / 边界\n")
	b.WriteString("- WHEN: 适用于与本次证据 receipts 高度相似的任务场景。\n")
	b.WriteString("- WHEN NOT: 不确定时先小范围验证；避免在未知 repo/关键路径上直接执行。\n\n")

	b.WriteString("## SOP 步骤（可复用流程）\n")
	b.WriteString("1. 明确目标与验收标准（需要哪些产物/报告）。\n")
	b.WriteString("2. 快速定位影响范围（关键文件/模块/入口）。\n")
	b.WriteString("3. 小步提交变更并持续自检（必要时生成测试报告文件作为证据）。\n")
	b.WriteString("4. 失败时收敛问题（缩小范围、回滚到上一步、记录差异）。\n")
	b.WriteString("5. 产出交付件：findings + trace +（可选）测试报告/输出文件路径。\n\n")

	b.WriteString("## 失败处理 / 常见坑\n")
	b.WriteString("- 如果方向错了：停止继续扩大修改，先回到需求与验收标准。\n")
	b.WriteString("- 如果卡住：把问题缩小到一个最小可复现例，并把证据写进 findings。\n")
	b.WriteString("- 如果需要 resume：基于上一轮 findings/trace 明确剩余工作再继续。\n\n")

	b.WriteString("## 验收方式（Evidence-first）\n")
	b.WriteString("- 必须有可复核证据：findings_path + trace_log_path。\n")
	b.WriteString("- 若需要命令验收：生成可读的测试报告文件并在 findings 中引用。\n\n")

	b.WriteString("## Evidence Receipts\n\n")
	for _, r := range rs {
		b.WriteString("- receipt_id=")
		b.WriteString(r.ReceiptID)
		b.WriteString("\n  - summary: ")
		b.WriteString(strings.TrimSpace(oneLineV1(r.Summary)))
		b.WriteString("\n")
		if strings.TrimSpace(r.Artifacts.FindingsPath) != "" {
			b.WriteString("  - findings_path: ")
			b.WriteString(strings.TrimSpace(r.Artifacts.FindingsPath))
			b.WriteString("\n")
		}
		if strings.TrimSpace(r.Artifacts.TraceLogPath) != "" {
			b.WriteString("  - trace_log_path: ")
			b.WriteString(strings.TrimSpace(r.Artifacts.TraceLogPath))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (s *Store) loadExistingEvidenceSignatures(principalID, dayKey string) map[string]bool {
	out := map[string]bool{}
	existing, err := s.ListSuggestions(ListSuggestionsQuery{
		PrincipalID:   principalID,
		DayKey:        dayKey,
		IncludeParked: true,
		Limit:         200,
	})
	if err != nil {
		return out
	}
	for _, sug := range existing {
		sig := evidenceSignatureV1(sug.EvidenceReceiptIDs)
		if sig != "" {
			out[sig] = true
		}
	}
	return out
}

func evidenceSignatureV1(ids []string) string {
	ids = dedupStrings(ids)
	if len(ids) == 0 {
		return ""
	}
	sort.Strings(ids)
	return strings.Join(ids, ";")
}

func oneLineV1(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}
