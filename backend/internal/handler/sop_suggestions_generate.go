package handler

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

type generateSuggestionsRequest struct {
	LookbackDays int `json:"lookback_days,omitempty"`
	Count        int `json:"count,omitempty"`
}

// GenerateSuggestions is a v1 best-effort generator that proposes a SOP suggestion
// from recent receipts. It is user-triggered and does not run automatically.
func GenerateSuggestions(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.WorkLedger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "work ledger not initialized"})
		return
	}

	principal := strings.TrimSpace(middleware.GetUserID(c))
	if principal == "" {
		principal = "local"
	}

	var req generateSuggestionsRequest
	_ = c.ShouldBindJSON(&req)

	lookback := req.LookbackDays
	if lookback <= 0 {
		lookback = 7
	}
	if lookback > 30 {
		lookback = 30
	}
	count := req.Count
	if count <= 0 {
		count = 1
	}
	if count > 3 {
		count = 3
	}

	after := time.Now().Add(-time.Duration(lookback) * 24 * time.Hour).UTC()
	receipts, err := rt.WorkLedger.ListReceipts(workledger.ListReceiptsQuery{
		PrincipalID:   principal,
		Status:        workledger.ReceiptStatusSucceeded,
		FinishedAfter: after,
		Limit:         500,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	usable := make([]workledger.Receipt, 0, len(receipts))
	for _, r := range receipts {
		if strings.TrimSpace(r.Artifacts.FindingsPath) == "" || strings.TrimSpace(r.Artifacts.TraceLogPath) == "" {
			continue
		}
		usable = append(usable, r)
	}
	if len(usable) < 2 {
		c.JSON(http.StatusOK, []workledger.Suggestion{})
		return
	}

	sort.SliceStable(usable, func(i, j int) bool { return usable[i].FinishedAt.After(usable[j].FinishedAt) })

	dayKey := workledger.DayKey(time.Now())
	existingSigs := loadExistingEvidenceSignatures(rt.WorkLedger, principal, dayKey)
	created := make([]workledger.Suggestion, 0, count)
	usedReceiptIDs := map[string]bool{}

	for len(created) < count {
		pair := pickNextPair(usable, usedReceiptIDs)
		if len(pair) < 2 {
			break
		}
		for _, r := range pair {
			usedReceiptIDs[r.ReceiptID] = true
		}

		evidence := []string{pair[0].ReceiptID, pair[1].ReceiptID}
		sig := evidenceSignature(evidence)
		if existingSigs[sig] {
			continue
		}
		title := deriveSuggestionTitle(pair)
		draft := buildDraftSkill(pair)
		scores := computeSuggestionScores(rt.WorkLedger, principal, dayKey, title, draft, evidence)

		sug, err := rt.WorkLedger.CreateSuggestion(workledger.CreateSuggestionInput{
			PrincipalID:        principal,
			WorkspaceRoot:      strings.TrimSpace(pair[0].WorkspaceRoot),
			Title:              title,
			Description:        "Auto-proposed from recent successful receipts (manual review required).",
			RiskNotes:          "This is a proposed SOP. Review carefully before approving.",
			EvidenceReceiptIDs: evidence,
			DraftSkill:         draft,
			Scores:             scores,
			Meta:              workledger.SuggestionMeta{DayKey: dayKey},
		})
		if err != nil {
			break
		}

		existingSigs[sig] = true
		_ = rt.WorkLedger.ApplyInboxCap(principal, dayKey, 10)
		created = append(created, sug)
	}

	c.JSON(http.StatusOK, created)
}

func pickNextPair(receipts []workledger.Receipt, used map[string]bool) []workledger.Receipt {
	out := make([]workledger.Receipt, 0, 2)
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

func deriveSuggestionTitle(rs []workledger.Receipt) string {
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

func buildDraftSkill(rs []workledger.Receipt) string {
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
		b.WriteString(strings.TrimSpace(oneLine(r.Summary)))
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

func loadExistingEvidenceSignatures(store *workledger.Store, principalID, dayKey string) map[string]bool {
	out := map[string]bool{}
	if store == nil {
		return out
	}
	existing, err := store.ListSuggestions(workledger.ListSuggestionsQuery{
		PrincipalID:   principalID,
		DayKey:        dayKey,
		IncludeParked: true,
		Limit:         200,
	})
	if err != nil {
		return out
	}
	for _, s := range existing {
		sig := evidenceSignature(s.EvidenceReceiptIDs)
		if sig != "" {
			out[sig] = true
		}
	}
	return out
}

func evidenceSignature(ids []string) string {
	ids = dedupStrings(ids)
	if len(ids) == 0 {
		return ""
	}
	sort.Strings(ids)
	return strings.Join(ids, ";")
}

func dedupStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
