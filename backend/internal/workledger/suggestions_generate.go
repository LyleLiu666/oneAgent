package workledger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
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
		draft := s.buildDraftSkillV1(pair)
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

		// Best-effort governance hints: does not affect generation success.
		_ = s.attachSuggestionGovernanceHintsV1(ctx, principal, sug.SuggestionID)
		if updated, err := s.GetSuggestion(sug.SuggestionID); err == nil {
			sug = updated
		}

		existingSigs[sig] = true
		_ = s.ApplyInboxCap(principal, dayKey, 10)
		created = append(created, sug)
	}

	return created, nil
}

func (s *Store) attachSuggestionGovernanceHintsV1(ctx context.Context, principalID string, suggestionID string) error {
	if s == nil {
		return errors.New("store is nil")
	}
	principalID = strings.TrimSpace(principalID)
	suggestionID = strings.TrimSpace(suggestionID)
	if principalID == "" || suggestionID == "" {
		return errors.New("principal_id and suggestion_id are required")
	}

	current, err := s.GetSuggestion(suggestionID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(current.PrincipalID) != principalID {
		return nil
	}

	all, err := s.ListSuggestions(ListSuggestionsQuery{
		PrincipalID:   principalID,
		IncludeParked: true,
		Limit:         200,
	})
	if err != nil {
		return err
	}

	candidates := make([]Suggestion, 0, len(all))
	for _, cand := range all {
		if cand.SuggestionID == current.SuggestionID {
			continue
		}
		candidates = append(candidates, cand)
	}

	sims := computeSimilarSuggestionsV1(current.Title, current.DraftSkill, candidates, 5)
	if len(sims) == 0 {
		return nil
	}

	ids := make([]string, 0, len(sims))
	for _, it := range sims {
		ids = append(ids, it.SuggestionID)
	}

	recommended := ""
	if sims[0].Similarity >= 0.5 {
		recommended = sims[0].SuggestionID
	}

	_, err = s.UpdateSuggestion(current.SuggestionID, func(cur *Suggestion) error {
		cur.Meta.SimilarSuggestionIDs = ids
		if strings.TrimSpace(recommended) != "" {
			cur.Meta.RecommendedMergeTargetID = recommended
		}
		return nil
	})
	return err
}

func pickNextPairV1(receipts []Receipt, used map[string]bool) []Receipt {
	const minSim = 0.1

	if len(receipts) < 2 {
		return nil
	}

	tokens := make([]map[string]struct{}, len(receipts))
	for i, r := range receipts {
		tokens[i] = toTokenSetPairingV1(r.Summary)
	}

	bestSim := 0.0
	bestI, bestJ := -1, -1
	bestRank := 1<<30

	for i := 0; i < len(receipts); i++ {
		if used[receipts[i].ReceiptID] {
			continue
		}
		for j := i + 1; j < len(receipts); j++ {
			if used[receipts[j].ReceiptID] {
				continue
			}
			sim := jaccardV1(tokens[i], tokens[j])
			if sim <= 0 {
				continue
			}

			// Prefer higher similarity; break ties by recency (lower i/j indices).
			rank := i*len(receipts) + j
			if sim > bestSim || (sim == bestSim && rank < bestRank) {
				bestSim = sim
				bestI, bestJ = i, j
				bestRank = rank
			}
		}
	}

	if bestI < 0 || bestJ < 0 || bestSim < minSim {
		return nil
	}
	return []Receipt{receipts[bestI], receipts[bestJ]}
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

func toTokenSetPairingV1(s string) map[string]struct{} {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return nil
	}

	out := make(map[string]struct{})
	var word strings.Builder
	flushWord := func() {
		if word.Len() >= 3 {
			out[word.String()] = struct{}{}
		}
		word.Reset()
	}

	for _, r := range s {
		if r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			word.WriteRune(r)
			continue
		}

		flushWord()

		if !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			continue
		}

		// Basic CJK-awareness: treat each non-ASCII letter as a token.
		if isCommonCJKStopCharV1(r) {
			continue
		}
		out[string(r)] = struct{}{}
	}

	flushWord()
	return out
}

func isCommonCJKStopCharV1(r rune) bool {
	switch r {
	case '的', '了', '是', '在', '和', '与', '及', '或', '也', '但', '就', '都', '而', '并', '对', '能', '这', '那', '个', '吗', '啊', '呢':
		return true
	default:
		return false
	}
}

func (s *Store) buildDraftSkillV1(rs []Receipt) string {
	var b strings.Builder

	theme := deriveReceiptThemeV1(rs)
	e := s.extractEvidenceFromReceiptsV1(rs)

	b.WriteString("# SOP (Proposed)\n\n")
	b.WriteString("## 使用时机 / 边界\n")
	if theme != "" {
		b.WriteString("- 主题（来自证据摘要）: ")
		b.WriteString(theme)
		b.WriteString("\n")
	}
	b.WriteString("- WHEN: 适用于与本次证据 receipts 相似的任务场景（优先复用“流水账”中的已验证步骤）。\n")
	b.WriteString("- WHEN NOT: 不确定时先小范围验证；避免在未知 repo/关键路径上直接执行。\n\n")

	b.WriteString("## SOP 步骤（可复用流程）\n")
	if len(e.TimelineSteps) > 0 {
		for i, step := range e.TimelineSteps {
			fmt.Fprintf(&b, "%d. %s\n", i+1, step)
		}
		b.WriteString("\n")
	} else {
		b.WriteString("1. 明确目标与验收标准（需要哪些产物/报告）。\n")
		b.WriteString("2. 快速定位影响范围（关键文件/模块/入口）。\n")
		b.WriteString("3. 小步提交变更并持续自检（必要时生成测试报告文件作为证据）。\n")
		b.WriteString("4. 失败时收敛问题（缩小范围、回滚到上一步、记录差异）。\n")
		b.WriteString("5. 产出交付件：findings + trace +（可选）测试报告/输出文件路径。\n\n")
	}

	b.WriteString("## 失败处理 / 常见坑\n")
	if len(e.Pitfalls) > 0 {
		for _, pitfall := range e.Pitfalls {
			b.WriteString("- ")
			b.WriteString(pitfall)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	} else {
		b.WriteString("- 如果方向错了：停止继续扩大修改，先回到需求与验收标准。\n")
		b.WriteString("- 如果卡住：把问题缩小到一个最小可复现例，并把证据写进 findings。\n")
		b.WriteString("- 如果需要 resume：基于上一轮 findings/trace 明确剩余工作再继续。\n\n")
	}

	b.WriteString("## 验收方式（Evidence-first）\n")
	b.WriteString("- 必须有可复核证据：findings_path + trace_log_path。\n")
	b.WriteString("- 若需要命令验收：生成可读的测试报告文件并在 findings 中引用。\n\n")

	if len(e.ChangedFiles) > 0 {
		b.WriteString("## 涉及文件（自动提取）\n")
		for _, file := range e.ChangedFiles {
			b.WriteString("- ")
			b.WriteString(file)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

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
		if strings.TrimSpace(r.Artifacts.TestReportPath) != "" {
			b.WriteString("  - test_report_path: ")
			b.WriteString(strings.TrimSpace(r.Artifacts.TestReportPath))
			b.WriteString("\n")
		}
		if strings.TrimSpace(r.Artifacts.DiffPatchPath) != "" {
			b.WriteString("  - diff_patch_path: ")
			b.WriteString(strings.TrimSpace(r.Artifacts.DiffPatchPath))
			b.WriteString("\n")
		}
		if strings.TrimSpace(r.Artifacts.ChangedFilesPath) != "" {
			b.WriteString("  - changed_files_path: ")
			b.WriteString(strings.TrimSpace(r.Artifacts.ChangedFilesPath))
			b.WriteString("\n")
		}
	}
	return b.String()
}

type extractedEvidenceV1 struct {
	TimelineSteps []string
	Pitfalls      []string
	ChangedFiles  []string
}

func (s *Store) extractEvidenceFromReceiptsV1(rs []Receipt) extractedEvidenceV1 {
	steps := make([]string, 0, 16)
	pitfalls := make([]string, 0, 16)
	changed := make([]string, 0, 24)

	for _, r := range rs {
		findings := s.readTextArtifactV1(strings.TrimSpace(r.Artifacts.FindingsPath), r)
		if findings == "" {
			continue
		}
		parsed := parseFindingsMarkdownV1(findings)
		steps = append(steps, parsed.Timeline...)
		pitfalls = append(pitfalls, parsed.Findings...)
		changed = append(changed, parsed.ChangedFiles...)
	}

	return extractedEvidenceV1{
		TimelineSteps: dedupAndLimitV1(steps, 8),
		Pitfalls:      dedupAndLimitV1(pitfalls, 8),
		ChangedFiles:  dedupAndLimitV1(changed, 12),
	}
}

type parsedFindingsV1 struct {
	Timeline     []string
	Findings     []string
	ChangedFiles []string
}

func parseFindingsMarkdownV1(md string) parsedFindingsV1 {
	var out parsedFindingsV1
	section := ""
	for _, raw := range strings.Split(md, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "##") {
			head := strings.TrimSpace(strings.TrimLeft(line, "#"))
			headLower := strings.ToLower(head)
			switch {
			case strings.Contains(headLower, "流水账") || strings.Contains(headLower, "timeline"):
				section = "timeline"
			case strings.Contains(headLower, "findings"):
				section = "findings"
			case strings.Contains(headLower, "变更文件") || strings.Contains(headLower, "changed files"):
				section = "changed"
			default:
				section = ""
			}
			continue
		}
		if section == "" {
			continue
		}
		item := ""
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			item = strings.TrimSpace(line[2:])
		}
		if item == "" {
			continue
		}
		switch section {
		case "timeline":
			out.Timeline = append(out.Timeline, item)
		case "findings":
			out.Findings = append(out.Findings, item)
		case "changed":
			out.ChangedFiles = append(out.ChangedFiles, item)
		}
	}
	return out
}

func dedupAndLimitV1(items []string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, minInt(limit, len(items)))
	for _, raw := range items {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		key = strings.Join(strings.Fields(key), " ")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func deriveReceiptThemeV1(rs []Receipt) string {
	if len(rs) == 0 {
		return ""
	}
	a := strings.TrimSpace(oneLineV1(rs[0].Summary))
	if a == "" {
		return ""
	}
	if len(rs) == 1 {
		return a
	}
	b := strings.TrimSpace(oneLineV1(rs[1].Summary))
	if b == "" || b == a {
		return a
	}
	theme := a + " / " + b
	r := []rune(theme)
	if len(r) > 120 {
		return string(r[:120]) + "…"
	}
	return theme
}

func (s *Store) readTextArtifactV1(path string, receipt Receipt) string {
	path = strings.TrimSpace(path)
	if path == "" || s == nil {
		return ""
	}

	candidate := path
	if !filepath.IsAbs(candidate) {
		if strings.TrimSpace(receipt.WorkspaceRoot) != "" {
			candidate = filepath.Join(receipt.WorkspaceRoot, candidate)
		} else {
			candidate = filepath.Join(s.baseDir, candidate)
		}
	}
	candidate = filepath.Clean(candidate)

	if !s.isAllowedArtifactPathV1(candidate, receipt) {
		return ""
	}

	data, err := readFileMaxV1(candidate, 256*1024)
	if err != nil {
		return ""
	}
	return string(data)
}

func (s *Store) isAllowedArtifactPathV1(path string, receipt Receipt) bool {
	if s == nil {
		return false
	}
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return false
	}

	roots := []string{}
	if strings.TrimSpace(receipt.WorkspaceRoot) != "" {
		roots = append(roots, filepath.Clean(filepath.Join(receipt.WorkspaceRoot, ".oneagent")))
	}
	if strings.TrimSpace(s.baseDir) != "" {
		roots = append(roots, filepath.Clean(filepath.Join(s.baseDir, "..", "..")))
	}

	for _, root := range roots {
		if root == "" {
			continue
		}
		if isWithinDirV1(path, root) {
			return true
		}
	}
	return false
}

func isWithinDirV1(path string, root string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	root = filepath.Clean(strings.TrimSpace(root))
	if path == "" || root == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func readFileMaxV1(path string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, errors.New("maxBytes must be > 0")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := io.LimitReader(f, maxBytes+1)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		data = data[:maxBytes]
	}
	return data, nil
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
