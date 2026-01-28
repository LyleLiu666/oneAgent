package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

const (
	ToolIDEditV2 = "edit_v2"
)

type editV2Request struct {
	FilePath string `json:"filePath"`

	OldString string `json:"oldString"`
	NewString string `json:"newString"`

	Occurrence int `json:"occurrence,omitempty"` // 1-based

	BeforeAnchor string `json:"before_anchor,omitempty"`
	AfterAnchor  string `json:"after_anchor,omitempty"`

	ExpectedReplacements *int `json:"expected_replacements,omitempty"` // nil=default(1), 0=ignore, otherwise must match

	Preconditions *FilePreconditions `json:"preconditions,omitempty"`
}

type editV2Result struct {
	OK bool `json:"ok"`

	FilePath string `json:"file_path,omitempty"`

	Replacements int `json:"replacements,omitempty"`
	StartLine    int `json:"start_line,omitempty"` // 1-based
	EndLine      int `json:"end_line,omitempty"`   // 1-based

	MatchedCount int `json:"matched_count,omitempty"`
	Strategy     string `json:"strategy,omitempty"`

	DiffPreview string `json:"diff_preview,omitempty"`
	DiffCapped  bool   `json:"diff_capped,omitempty"`

	PreconditionFailed bool   `json:"precondition_failed,omitempty"`
	ExpectedSHA256     string `json:"expected_sha256,omitempty"`
	GotSHA256          string `json:"got_sha256,omitempty"`

	Error       string `json:"error,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
	Diagnostics string `json:"diagnostics,omitempty"`
}

func editV2Definition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "edit_v2",
			Description: "确定性编辑（无 silent success）。支持 occurrence/anchors/expected_replacements/OCC，并返回可解释证据（matched_range + diff_preview）。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filePath": map[string]any{"type": "string"},
					"oldString": map[string]any{"type": "string"},
					"newString": map[string]any{"type": "string"},
					"occurrence": map[string]any{
						"type":        "integer",
						"description": "（可选）替换第 N 次匹配（1-based）。",
					},
					"before_anchor": map[string]any{
						"type":        "string",
						"description": "（可选）用于消歧：匹配必须出现在 before_anchor 之后。",
					},
					"after_anchor": map[string]any{
						"type":        "string",
						"description": "（可选）用于消歧：匹配必须出现在 after_anchor 之前。",
					},
					"expected_replacements": map[string]any{
						"type":        "integer",
						"description": "（可选）期望替换次数；0 表示不检查。",
					},
					"preconditions": map[string]any{
						"type":        "object",
						"description": "（可选）条件写入（OCC）。",
						"properties": map[string]any{
							"expected_exists": map[string]any{"type": "boolean"},
							"expected_sha256": map[string]any{"type": "string"},
						},
						"additionalProperties": false,
					},
				},
				"required":             []string{"filePath", "oldString", "newString"},
				"additionalProperties": false,
			},
		},
	}
	return newDefinition(ToolIDEditV2, spec, runEditV2Tool)
}

func runEditV2Tool(ctx context.Context, raw json.RawMessage) (any, error) {
	var req editV2Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	req.FilePath = strings.TrimSpace(req.FilePath)
	if req.FilePath == "" {
		return nil, errors.New("filePath 不能为空")
	}
	if req.OldString == "" {
		return nil, errors.New("oldString 不能为空")
	}
	// newString can be empty (delete).

	dec, err := RequirePolicy(ctx, ToolIDEditV2)
	if err != nil {
		return nil, err
	}

	root, target, err := resolvePathForWrite(ctx, req.FilePath)
	if err != nil {
		return nil, err
	}
	if root != "" {
		if rel, err := filepath.Rel(root, target); err == nil {
			relSlash := filepath.ToSlash(rel)
			relSlash = strings.TrimPrefix(relSlash, "./")
			if err := EnforceFileScope(root, relSlash, dec); err != nil {
				return nil, err
			}
		}
	}

	if _, err := os.Stat(target); err != nil {
		return nil, err
	}

	preconditions := req.Preconditions
	if preconditions == nil {
		if expected, ok := occExpectedSHA256(ctx, target); ok {
			preconditions = &FilePreconditions{ExpectedSHA256: expected}
		}
	}
	if err := checkFilePreconditions(target, preconditions); err != nil {
		expected := ""
		if preconditions != nil {
			expected = strings.TrimSpace(preconditions.ExpectedSHA256)
		}
		got := ""
		if sha, shaErr := fileSHA256Hex(target); shaErr == nil {
			got = sha
		}
		return editV2Result{
			OK:                false,
			FilePath:           req.FilePath,
			PreconditionFailed: true,
			ExpectedSHA256:     expected,
			GotSHA256:          got,
			Error:              err.Error(),
			Suggestion:         "The file changed since it was read; re-run read_file and retry with the new expected_sha256.",
		}, nil
	}

	rawBytes, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}

	original := string(rawBytes)
	useCRLF := strings.Contains(original, "\r\n")
	normalized := strings.ReplaceAll(original, "\r\n", "\n")
	oldNorm := strings.ReplaceAll(req.OldString, "\r\n", "\n")
	newNorm := strings.ReplaceAll(req.NewString, "\r\n", "\n")

	matches := findAllOccurrences(normalized, oldNorm)
	filtered, strategy, diag := filterMatchesByAnchors(normalized, matches, oldNorm, strings.ReplaceAll(req.BeforeAnchor, "\r\n", "\n"), strings.ReplaceAll(req.AfterAnchor, "\r\n", "\n"))

	expected := 1
	if req.ExpectedReplacements != nil {
		expected = *req.ExpectedReplacements
	}

	selected := -1
	switch {
	case req.Occurrence > 0:
		if req.Occurrence > len(filtered) {
			return editV2Result{
				OK:           false,
				FilePath:      req.FilePath,
				MatchedCount:  len(filtered),
				Strategy:      "occurrence",
				Error:         fmt.Sprintf("occurrence=%d out of range (matches=%d)", req.Occurrence, len(filtered)),
				Suggestion:    "Use read_file to inspect contexts and adjust occurrence/anchors.",
				Diagnostics:   diag,
			}, nil
		}
		selected = filtered[req.Occurrence-1]
		strategy = "occurrence"
	default:
		if len(filtered) == 0 {
			return editV2Result{
				OK:          false,
				FilePath:     req.FilePath,
				MatchedCount: 0,
				Strategy:     "no_match",
				Error:        "no match found",
				Suggestion:   "Call read_file to confirm the exact oldString (including whitespace/newlines).",
				Diagnostics:  diag,
			}, nil
		}
		if len(filtered) != 1 {
			return editV2Result{
				OK:          false,
				FilePath:     req.FilePath,
				MatchedCount: len(filtered),
				Strategy:     "unique_match_required",
				Error:        fmt.Sprintf("ambiguous match: matches=%d (provide occurrence or anchors)", len(filtered)),
				Suggestion:   "Call read_file around each candidate and provide occurrence/before_anchor/after_anchor to disambiguate.",
				Diagnostics:  diag,
			}, nil
		}
		selected = filtered[0]
		if strategy == "" {
			strategy = "single_match"
		}
	}

	if selected < 0 || selected > len(normalized) {
		return editV2Result{OK: false, FilePath: req.FilePath, Error: "internal error: invalid match index"}, nil
	}

	updated := normalized[:selected] + newNorm + normalized[selected+len(oldNorm):]
	if useCRLF {
		updated = strings.ReplaceAll(updated, "\n", "\r\n")
	}

	startLine, endLine := rangeLines(normalized, selected, selected+len(oldNorm))

	if expected != 0 && expected != 1 {
		return editV2Result{
			OK:           false,
			FilePath:      req.FilePath,
			MatchedCount:  len(filtered),
			Strategy:      strategy,
			Error:         fmt.Sprintf("expected_replacements=%d not supported in v1 (only 1)", expected),
			Suggestion:    "Use occurrence/anchors to target a single replacement per call.",
			Diagnostics:   diag,
		}, nil
	}

	if err := os.WriteFile(target, []byte(updated), 0o644); err != nil {
		return nil, err
	}

	// Record OCC for subsequent calls.
	if OCCFromContext(ctx) != nil {
		if sha, err := fileSHA256Hex(target); err == nil {
			occRecordSHA256(ctx, target, sha)
		}
	}

	diffPreview, capped := buildEditPreview(original, updated, startLine, endLine, 4096)

	return editV2Result{
		OK:          true,
		FilePath:     req.FilePath,
		Replacements: 1,
		StartLine:    startLine,
		EndLine:      endLine,
		MatchedCount: len(filtered),
		Strategy:     strategy,
		DiffPreview:  diffPreview,
		DiffCapped:   capped,
	}, nil
}

func findAllOccurrences(haystack, needle string) []int {
	if needle == "" || haystack == "" {
		return nil
	}
	out := make([]int, 0, 8)
	start := 0
	for {
		i := strings.Index(haystack[start:], needle)
		if i < 0 {
			break
		}
		pos := start + i
		out = append(out, pos)
		start = pos + len(needle)
	}
	return out
}

func filterMatchesByAnchors(content string, matches []int, needle, before, after string) ([]int, string, string) {
	if len(matches) == 0 {
		return nil, "no_match", "no matches found"
	}
	before = strings.TrimSpace(before)
	after = strings.TrimSpace(after)
	if before == "" && after == "" {
		return matches, "no_anchor", fmt.Sprintf("matches=%d (no anchors)", len(matches))
	}

	lo := 0
	hi := len(content)
	strategy := "anchor"
	diag := []string{fmt.Sprintf("matches=%d", len(matches))}

	if before != "" {
		bi := strings.LastIndex(content, before)
		if bi >= 0 {
			lo = bi + len(before)
			diag = append(diag, fmt.Sprintf("before_anchor_at=%d", bi))
		} else {
			diag = append(diag, "before_anchor_not_found")
		}
	}
	if after != "" {
		ai := strings.Index(content, after)
		if ai >= 0 {
			hi = ai
			diag = append(diag, fmt.Sprintf("after_anchor_at=%d", ai))
		} else {
			diag = append(diag, "after_anchor_not_found")
		}
	}

	filtered := make([]int, 0, len(matches))
	for _, m := range matches {
		if m < lo || m+len(needle) > hi {
			continue
		}
		filtered = append(filtered, m)
	}
	if len(filtered) == 0 {
		strategy = "anchor_no_candidate"
	}
	return filtered, strategy, strings.Join(diag, "; ")
}

func rangeLines(content string, start, end int) (int, int) {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if start > len(content) {
		start = len(content)
	}
	if end > len(content) {
		end = len(content)
	}
	startLine := 1 + strings.Count(content[:start], "\n")
	endLine := 1 + strings.Count(content[:end], "\n")
	return startLine, endLine
}

func buildEditPreview(before, after string, startLine, endLine int, maxBytes int) (string, bool) {
	if maxBytes <= 0 {
		maxBytes = 4096
	}
	linesBefore := strings.Split(before, "\n")
	linesAfter := strings.Split(after, "\n")

	// Grab a small window around the edited range.
	from := startLine - 3
	if from < 1 {
		from = 1
	}
	to := endLine + 3
	if to > len(linesBefore) {
		to = len(linesBefore)
	}
	if to > len(linesAfter) {
		to = len(linesAfter)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "--- before\n+++ after\n@@ lines %d-%d @@\n", from, to)
	for i := from; i <= to; i++ {
		if i-1 < len(linesBefore) {
			fmt.Fprintf(&b, "-%04d %s\n", i, linesBefore[i-1])
		}
		if i-1 < len(linesAfter) {
			fmt.Fprintf(&b, "+%04d %s\n", i, linesAfter[i-1])
		}
	}
	out := b.String()
	if len(out) <= maxBytes {
		return out, false
	}
	return out[:maxBytes] + "\n...(truncated)\n", true
}
