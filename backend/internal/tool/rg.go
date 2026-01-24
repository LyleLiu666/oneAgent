package tool

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

const (
	defaultRgMaxResults       = 50
	maxRgMaxResults           = 200
	maxRgStdoutLineBytes      = 2 * 1024 * 1024
	maxRgCapturedStderr       = 16 * 1024
	defaultRgSearchPath       = "."
	defaultRgMaxLineRunes     = 400
	defaultRgMaxSubmatchRunes = 200
	defaultRgMaxOutputBytes   = 64 * 1024
	ripgrepExecutableName     = "rg"
)

type rgToolRequest struct {
	Pattern      string `json:"pattern"`
	Path         string `json:"path,omitempty"`
	MaxResults   int    `json:"max_results,omitempty"`
	FixedStrings bool   `json:"fixed_strings,omitempty"`
}

type RgSubmatch struct {
	Match              string `json:"match"`
	Start              int    `json:"start"`
	End                int    `json:"end"`
	MatchTruncated     bool   `json:"match_truncated,omitempty"`
	OriginalMatchRunes int    `json:"original_match_runes,omitempty"`
}

type RgMatch struct {
	Path              string       `json:"path"`
	LineNumber        int          `json:"line_number"`
	Lines             string       `json:"lines"`
	LinesTruncated    bool         `json:"lines_truncated,omitempty"`
	OriginalLineRunes int          `json:"original_line_runes,omitempty"`
	Submatches        []RgSubmatch `json:"submatches,omitempty"`
}

type RgToolResult struct {
	Available          bool      `json:"available"`
	NotAvailableReason string    `json:"not_available_reason,omitempty"`
	Root               string    `json:"root"`
	Path               string    `json:"path"`
	Pattern            string    `json:"pattern"`
	MaxResults         int       `json:"max_results"`
	FixedStrings       bool      `json:"fixed_strings,omitempty"`
	MaxLineRunes       int       `json:"max_line_runes"`
	MaxOutputBytes     int       `json:"max_output_bytes"`
	Matches            []RgMatch `json:"matches"`
	Truncated          bool      `json:"truncated,omitempty"`
	TruncatedReason    string    `json:"truncated_reason,omitempty"`
	DurationMs         int64     `json:"duration_ms"`
	Stderr             string    `json:"stderr,omitempty"`
}

func rgDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name: "rg",
			Description: "使用 ripgrep (rg) 在沙箱根目录 $BASH_ROOT_DIR 内进行高速本地搜索（支持正则）。" +
				"不经过 shell，因此允许 `$` 等正则符号；搜索路径必须在沙箱内。" +
				"如果系统未安装 rg，将返回 available=false（不会报错），你可以自行决定改用 bash 或其它方式继续。" +
				"参数：pattern(必填), path(可选, 默认 '.'), max_results(可选, 默认 50, 最大 200), fixed_strings(可选)。" +
				"返回：匹配列表（相对 root 的文件路径、行号、行文本、submatches），并对单行/总输出做截断以避免返回过大内容。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "要搜索的正则表达式（或 fixed_strings=true 时为字面量字符串）。",
					},
					"path": map[string]any{
						"type":        "string",
						"description": "搜索路径（相对 $BASH_ROOT_DIR，或为其内部的绝对路径），默认 '.'。",
					},
					"max_results": map[string]any{
						"type":        "integer",
						"description": "最多返回多少条匹配（默认 50，最大 200）。达到上限会提前终止搜索并标记 truncated=true。",
						"minimum":     1,
						"maximum":     maxRgMaxResults,
					},
					"fixed_strings": map[string]any{
						"type":        "boolean",
						"description": "true=按字面量字符串搜索（等价于 rg --fixed-strings）。",
					},
				},
				"required":             []string{"pattern"},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDRg, spec, runRgTool)
}

type cappedBuffer struct {
	limit     int
	truncated bool
	buf       bytes.Buffer
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) <= remaining {
		_, _ = b.buf.Write(p)
		return len(p), nil
	}
	_, _ = b.buf.Write(p[:remaining])
	b.truncated = true
	return len(p), nil
}

func runRgTool(ctx context.Context, raw json.RawMessage) (any, error) {
	var req rgToolRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	pattern := strings.TrimSpace(req.Pattern)
	if pattern == "" {
		return nil, errors.New("pattern is required")
	}

	root, err := resolveSmartEditRoot()
	if err != nil {
		return nil, err
	}

	pathValue := strings.TrimSpace(req.Path)
	if pathValue == "" {
		pathValue = defaultRgSearchPath
	}
	target, err := resolvePathWithinRoot(root, pathValue)
	if err != nil {
		return nil, err
	}

	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = defaultRgMaxResults
	}
	if maxResults > maxRgMaxResults {
		maxResults = maxRgMaxResults
	}

	maxLineRunes := defaultRgMaxLineRunes
	maxSubmatchRunes := defaultRgMaxSubmatchRunes
	maxOutputBytes := defaultRgMaxOutputBytes

	rgPath, err := exec.LookPath(ripgrepExecutableName)
	if err != nil {
		return RgToolResult{
			Available:          false,
			NotAvailableReason: err.Error(),
			Root:               root,
			Path:               target,
			Pattern:            pattern,
			MaxResults:         maxResults,
			FixedStrings:       req.FixedStrings,
			MaxLineRunes:       maxLineRunes,
			MaxOutputBytes:     maxOutputBytes,
			Matches:            []RgMatch{},
			DurationMs:         0,
		}, nil
	}

	args := []string{"--json", "--no-config"}
	if req.FixedStrings {
		args = append(args, "--fixed-strings")
	}
	args = append(args, "--", pattern, target)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(runCtx, rgPath, args...)
	cmd.Dir = root

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	startedAt := time.Now()
	if err := cmd.Start(); err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return nil, fmt.Errorf("rg not available: %v", execErr)
		}
		return nil, err
	}

	stderrBuf := &cappedBuffer{limit: maxRgCapturedStderr}
	stderrDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(stderrBuf, stderr)
		close(stderrDone)
	}()

	type rgEvent struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	type rgMatchData struct {
		Path struct {
			Text string `json:"text"`
		} `json:"path"`
		Lines struct {
			Text string `json:"text"`
		} `json:"lines"`
		LineNumber int `json:"line_number"`
		Submatches []struct {
			Match struct {
				Text string `json:"text"`
			} `json:"match"`
			Start int `json:"start"`
			End   int `json:"end"`
		} `json:"submatches"`
	}

	matches := make([]RgMatch, 0, min(maxResults, 32))
	truncated := false
	truncatedReason := ""
	approxOutputBytes := 0

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), maxRgStdoutLineBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event rgEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			cancel()
			_ = cmd.Wait()
			<-stderrDone
			return nil, fmt.Errorf("failed to parse rg json output: %w", err)
		}

		if event.Type != "match" {
			continue
		}

		var data rgMatchData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			cancel()
			_ = cmd.Wait()
			<-stderrDone
			return nil, fmt.Errorf("failed to parse rg match event: %w", err)
		}

		submatches := make([]RgSubmatch, 0, len(data.Submatches))
		for _, sm := range data.Submatches {
			matchText, matchTruncated, originalMatchRunes := truncateToRunes(sm.Match.Text, maxSubmatchRunes)
			if !matchTruncated {
				originalMatchRunes = 0
			}
			submatches = append(submatches, RgSubmatch{
				Match:              matchText,
				Start:              sm.Start,
				End:                sm.End,
				MatchTruncated:     matchTruncated,
				OriginalMatchRunes: originalMatchRunes,
			})
		}

		matchAbsPath, err := resolvePathWithinRoot(root, data.Path.Text)
		if err != nil {
			cancel()
			_ = cmd.Wait()
			<-stderrDone
			return nil, fmt.Errorf("rg returned path outside root: %w", err)
		}
		matchRelPath, err := filepath.Rel(root, matchAbsPath)
		if err != nil {
			cancel()
			_ = cmd.Wait()
			<-stderrDone
			return nil, fmt.Errorf("failed to compute relative match path: %w", err)
		}
		matchRelPath = filepath.Clean(matchRelPath)

		linesText := strings.TrimSuffix(data.Lines.Text, "\n")
		truncatedLinesText, linesTruncated, originalLineRunes := truncateToRunes(linesText, maxLineRunes)
		if !linesTruncated {
			originalLineRunes = 0
		}
		linesText = truncatedLinesText

		nextBytes := len(matchRelPath) + len(linesText)
		for _, sm := range submatches {
			nextBytes += len(sm.Match)
		}
		if approxOutputBytes+nextBytes > maxOutputBytes {
			truncated = true
			truncatedReason = "max_output_bytes"
			cancel()
			break
		}

		matches = append(matches, RgMatch{
			Path:              matchRelPath,
			LineNumber:        data.LineNumber,
			Lines:             linesText,
			LinesTruncated:    linesTruncated,
			OriginalLineRunes: originalLineRunes,
			Submatches:        submatches,
		})
		approxOutputBytes += nextBytes

		if len(matches) >= maxResults {
			truncated = true
			if truncatedReason == "" {
				truncatedReason = "max_results"
			}
			cancel()
			break
		}
	}
	scanErr := scanner.Err()

	waitErr := cmd.Wait()
	<-stderrDone

	duration := time.Since(startedAt)

	if scanErr != nil {
		return nil, scanErr
	}

	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode := exitErr.ExitCode()
			// rg: 0=match found, 1=no matches, 2=error.
			switch {
			case exitCode == 1:
				// No matches is not an error.
			case truncated && (exitCode == -1 || errors.Is(runCtx.Err(), context.Canceled)):
				// Canceled due to max_results.
			default:
				msg := strings.TrimSpace(stderrBuf.buf.String())
				if msg == "" {
					msg = exitErr.Error()
				}
				return nil, fmt.Errorf("rg failed (exit %d): %s", exitCode, msg)
			}
		} else if truncated && errors.Is(runCtx.Err(), context.Canceled) {
			// Canceled due to max_results, ignore.
		} else {
			return nil, waitErr
		}
	}

	return RgToolResult{
		Available:       true,
		Root:            root,
		Path:            target,
		Pattern:         pattern,
		MaxResults:      maxResults,
		FixedStrings:    req.FixedStrings,
		MaxLineRunes:    maxLineRunes,
		MaxOutputBytes:  maxOutputBytes,
		Matches:         matches,
		Truncated:       truncated,
		TruncatedReason: truncatedReason,
		DurationMs:      duration.Milliseconds(),
		Stderr:          strings.TrimSpace(stderrBuf.buf.String()),
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
