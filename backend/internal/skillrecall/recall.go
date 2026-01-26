package skillrecall

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/skill"
)

type Candidate struct {
	Skill         skill.Skill `json:"skill"`
	Score         int         `json:"score"`
	MetadataScore int         `json:"metadata_score,omitempty"`
	ContentScore  int         `json:"content_score,omitempty"`
}

type Result struct {
	Backend            string      `json:"backend"` // "rg" | "grep" | "go" | "none"
	NotAvailableReason string      `json:"not_available_reason,omitempty"`
	Candidates         []Candidate `json:"candidates"`
	DurationMs         int64       `json:"duration_ms"`
}

type Options struct {
	MaxResults int
	Timeout    time.Duration
}

type LookPathFunc func(string) (string, error)

var tokenRe = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9_-]{2,}`)

func ParseExplicitSkill(catalog *skill.Catalog, query string) (skill.Skill, bool) {
	if catalog == nil || len(catalog.Skills) == 0 {
		return skill.Skill{}, false
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return skill.Skill{}, false
	}

	matches := tokenRe.FindAllString(query, -1)
	if len(matches) == 0 {
		return skill.Skill{}, false
	}

	var (
		best      skill.Skill
		bestToken string
		found     bool
	)

	for _, tok := range matches {
		norm := skill.NormalizeName(tok)
		if norm == "" {
			continue
		}
		s, ok := catalog.ByID(norm)
		if !ok {
			s, ok = catalog.ByName(norm)
		}
		if !ok {
			continue
		}
		if !found || len(norm) > len(bestToken) || (len(norm) == len(bestToken) && s.ID < best.ID) {
			best = s
			bestToken = norm
			found = true
		}
	}

	return best, found
}

func Search(ctx context.Context, catalog *skill.Catalog, query string, opts Options, lookPath LookPathFunc) (Result, error) {
	query = strings.TrimSpace(query)
	if query == "" || catalog == nil || len(catalog.Skills) == 0 {
		return Result{Backend: "none", Candidates: nil}, nil
	}

	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = 8
	}
	if maxResults > 50 {
		maxResults = 50
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	if lookPath == nil {
		lookPath = exec.LookPath
	}

	started := time.Now()

	meta := make(map[string]int, len(catalog.Skills))
	for _, s := range catalog.Skills {
		meta[s.ID] = scoreByMetadata(s, query)
	}

	paths := make([]string, 0, len(catalog.Skills))
	for _, s := range catalog.Skills {
		paths = append(paths, s.Path)
	}

	content, backend, reason := countMatches(ctx, query, paths, timeout, lookPath)

	cands := make([]Candidate, 0, len(catalog.Skills))
	for _, s := range catalog.Skills {
		metaScore := meta[s.ID]
		contentScore := content[s.Path]
		score := metaScore + contentScore
		cands = append(cands, Candidate{
			Skill:         s,
			Score:         score,
			MetadataScore: metaScore,
			ContentScore:  contentScore,
		})
	}

	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].Score != cands[j].Score {
			return cands[i].Score > cands[j].Score
		}
		if cands[i].Skill.ID != cands[j].Skill.ID {
			return cands[i].Skill.ID < cands[j].Skill.ID
		}
		return cands[i].Skill.Path < cands[j].Skill.Path
	})

	if len(cands) > maxResults {
		cands = cands[:maxResults]
	}

	return Result{
		Backend:            backend,
		NotAvailableReason: reason,
		Candidates:         cands,
		DurationMs:         time.Since(started).Milliseconds(),
	}, nil
}

func scoreByMetadata(s skill.Skill, query string) int {
	q := strings.ToLower(query)
	score := 0

	name := strings.ToLower(s.Name)
	if name != "" && strings.Contains(q, name) {
		score += 80
	}
	if s.ID != "" && strings.Contains(q, s.ID) {
		score += 120
	}

	desc := strings.ToLower(s.Description)
	if desc != "" && strings.Contains(desc, q) {
		score += 30
	}

	for _, tag := range s.Tags {
		tagLower := strings.ToLower(strings.TrimSpace(tag))
		if tagLower != "" && strings.Contains(q, tagLower) {
			score += 20
		}
	}
	for _, kw := range s.Keywords {
		kwLower := strings.ToLower(strings.TrimSpace(kw))
		if kwLower != "" && strings.Contains(q, kwLower) {
			score += 20
		}
	}

	return score
}

func countMatches(ctx context.Context, query string, files []string, timeout time.Duration, lookPath LookPathFunc) (map[string]int, string, string) {
	out := make(map[string]int, 64)
	query = truncateRunes(strings.TrimSpace(query), 200)
	if query == "" || len(files) == 0 {
		return out, "none", ""
	}

	diskFiles := make([]string, 0, len(files))
	builtinFiles := make([]string, 0, 8)
	for _, f := range files {
		if skill.IsBuiltinPath(f) {
			builtinFiles = append(builtinFiles, f)
			continue
		}
		diskFiles = append(diskFiles, f)
	}

	for _, f := range builtinFiles {
		data, err := skill.ReadSkillFile(f, 512*1024)
		if err != nil {
			continue
		}
		if n := countFixedStringFold(string(data), query, 50); n > 0 {
			out[f] = n
		}
	}

	if len(diskFiles) == 0 {
		return out, "none", ""
	}

	contentCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rgReason := ""
	if rgPath, err := lookPath("rg"); err == nil && strings.TrimSpace(rgPath) != "" {
		if counts, err := runCountMatches(contentCtx, rgPath, rgArgsPrefix(), query, diskFiles); err == nil {
			for p, n := range counts {
				out[p] += n
			}
			return out, "rg", ""
		} else {
			rgReason = err.Error()
		}
	} else if err != nil {
		rgReason = err.Error()
	}

	grepReason := ""
	if grepPath, err := lookPath("grep"); err == nil && strings.TrimSpace(grepPath) != "" {
		if counts, err := runCountMatches(contentCtx, grepPath, grepArgsPrefix(), query, diskFiles); err == nil {
			for p, n := range counts {
				out[p] += n
			}
			return out, "grep", rgReason
		} else {
			grepReason = err.Error()
		}
	} else if err != nil {
		grepReason = err.Error()
	}

	reasonParts := make([]string, 0, 2)
	if strings.TrimSpace(rgReason) != "" {
		reasonParts = append(reasonParts, "rg not available ("+rgReason+")")
	} else {
		reasonParts = append(reasonParts, "rg not available")
	}
	if strings.TrimSpace(grepReason) != "" {
		reasonParts = append(reasonParts, "grep not available ("+grepReason+")")
	} else {
		reasonParts = append(reasonParts, "grep not available")
	}
	reason := strings.Join(reasonParts, "; ")

	for _, f := range diskFiles {
		select {
		case <-contentCtx.Done():
			return out, "go", reason
		default:
		}
		data, err := skill.ReadSkillFile(f, 512*1024)
		if err != nil {
			continue
		}
		if n := countFixedStringFold(string(data), query, 50); n > 0 {
			out[f] += n
		}
	}

	return out, "go", reason
}

func countFixedStringFold(haystack string, needle string, maxCount int) int {
	needle = strings.TrimSpace(needle)
	if needle == "" || haystack == "" {
		return 0
	}

	if maxCount <= 0 {
		maxCount = 50
	}

	h := strings.ToLower(haystack)
	n := strings.ToLower(needle)

	count := 0
	for {
		i := strings.Index(h, n)
		if i < 0 {
			break
		}
		count++
		if count >= maxCount {
			break
		}
		h = h[i+len(n):]
	}

	return count
}

func rgArgsPrefix() []string {
	return []string{
		"--count-matches",
		"--no-config",
		"--fixed-strings",
		"--ignore-case",
		"--max-count", "50",
		"--",
	}
}

func grepArgsPrefix() []string {
	return []string{
		"-c",
		"-H",
		"-F",
		"-i",
		"--binary-files=without-match",
		"--",
	}
}

func runCountMatches(ctx context.Context, exe string, prefix []string, query string, files []string) (map[string]int, error) {
	out := make(map[string]int, 64)

	const chunkSize = 200
	for start := 0; start < len(files); start += chunkSize {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("command timed out")
		default:
		}

		end := start + chunkSize
		if end > len(files) {
			end = len(files)
		}
		chunk := files[start:end]

		args := make([]string, 0, len(prefix)+1+len(chunk))
		args = append(args, prefix...)
		args = append(args, query)
		args = append(args, chunk...)

		cmd := exec.CommandContext(ctx, exe, args...)
		stderrBuf := &cappedBuffer{limit: 16 * 1024}
		cmd.Stderr = stderrBuf
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}

		if err := cmd.Start(); err != nil {
			return nil, err
		}

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			colon := strings.LastIndex(line, ":")
			if colon <= 0 {
				continue
			}
			pathPart := strings.TrimSpace(line[:colon])
			countPart := strings.TrimSpace(line[colon+1:])
			n, err := strconv.Atoi(countPart)
			if err != nil || n <= 0 {
				continue
			}
			out[filepath.Clean(pathPart)] += n
		}
		if err := scanner.Err(); err != nil {
			_ = cmd.Wait()
			return nil, err
		}

		waitErr := cmd.Wait()

		if waitErr != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf("command timed out")
			}
			var exitErr *exec.ExitError
			if errors.As(waitErr, &exitErr) {
				code := exitErr.ExitCode()
				// 1 means "no matches", which is not an error for recall.
				if code == 1 {
					continue
				}
			}
			msg := strings.TrimSpace(stderrBuf.String())
			if msg != "" {
				return nil, fmt.Errorf("%v: %s", waitErr, msg)
			}
			return nil, waitErr
		}
	}

	return out, nil
}

type cappedBuffer struct {
	limit     int
	truncated bool
	buf       strings.Builder
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b == nil || b.limit <= 0 {
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

func (b *cappedBuffer) String() string {
	if b == nil {
		return ""
	}
	return b.buf.String()
}

func truncateRunes(value string, maxRunes int) string {
	if maxRunes <= 0 || value == "" {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}
