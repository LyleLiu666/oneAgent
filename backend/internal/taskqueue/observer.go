package taskqueue

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

type ObserveInput struct {
	TaskID    string
	AttemptID string

	WorkspaceRoot string
	Prompt        string

	Summary        string
	FindingsPath   string
	TraceLogPath   string
	TestReportPath string
}

type OutcomeObserver struct {
	Client llm.Client

	Model string

	MaxFindingsBytes  int
	MaxTraceTailBytes int
}

func (o *OutcomeObserver) Decide(ctx context.Context, in ObserveInput) (ObserverDecision, error) {
	if o == nil {
		return ObserverDecision{}, errors.New("observer is nil")
	}
	if o.Client == nil {
		return ObserverDecision{}, errors.New("observer client is required")
	}
	if strings.TrimSpace(in.TaskID) == "" {
		return ObserverDecision{}, errors.New("task_id is required")
	}
	if strings.TrimSpace(in.AttemptID) == "" {
		return ObserverDecision{}, errors.New("attempt_id is required")
	}
	if strings.TrimSpace(in.WorkspaceRoot) == "" {
		return ObserverDecision{}, errors.New("workspace_root is required")
	}
	if strings.TrimSpace(in.Prompt) == "" {
		return ObserverDecision{}, errors.New("prompt is required")
	}

	findingsMax := o.MaxFindingsBytes
	if findingsMax <= 0 {
		findingsMax = 128 * 1024
	}
	traceTailMax := o.MaxTraceTailBytes
	if traceTailMax <= 0 {
		traceTailMax = 32 * 1024
	}

	findingsText, findingsMeta := readFileHead(in.FindingsPath, findingsMax)
	traceTail, traceMeta := readFileTail(in.TraceLogPath, traceTailMax)
	testReportText, testReportMeta := readFileHead(in.TestReportPath, findingsMax)

system := strings.TrimSpace(`
You are an Outcome Observer for an autonomous coding agent.

Goal: Decide if the attempt satisfies the user's original expectations.

Rules:
- You MUST be evidence-based. Prefer citing file paths and excerpts from the provided artifacts.
- You MUST NOT rely solely on the presence/absence of TODOs or a plan checklist.
- You are read-only: you cannot run commands or assume tests were executed unless evidence is provided.
- If evidence is insufficient, FAIL and explain what evidence is missing.
- If pass=false, you MUST propose next_steps as an actionable plan for the next attempt.
- You SHOULD answer reasonable "follow-up questions" by choosing a path based on evidence, instead of asking the user.

Output:
Return ONLY an XML block:

<observer_decision>
  <pass>true</pass>
  <reason>...</reason>
  <evidence>
    <item>...</item>
  </evidence>
  <next_steps>...</next_steps>
  <questions_for_user>
    <item>...</item>
  </questions_for_user>
</observer_decision>
`)

	var user strings.Builder
	user.WriteString("## Task\n")
	user.WriteString(fmt.Sprintf("- task_id: %s\n", in.TaskID))
	user.WriteString(fmt.Sprintf("- attempt_id: %s\n", in.AttemptID))
	user.WriteString(fmt.Sprintf("- workspace_root: %s\n", in.WorkspaceRoot))
	user.WriteString("\n### User expectation (prompt)\n")
	user.WriteString(in.Prompt)
	user.WriteString("\n")

	if strings.TrimSpace(in.Summary) != "" {
		user.WriteString("\n### Attempt summary\n")
		user.WriteString(in.Summary)
		user.WriteString("\n")
	}

	user.WriteString("\n## Artifacts\n")
	user.WriteString(fmt.Sprintf("- findings_path: %s (%s)\n", in.FindingsPath, findingsMeta))
	user.WriteString(fmt.Sprintf("- trace_log_path: %s (%s)\n", in.TraceLogPath, traceMeta))
	user.WriteString(fmt.Sprintf("- test_report_path: %s (%s)\n", in.TestReportPath, testReportMeta))

	if strings.TrimSpace(findingsText) != "" {
		user.WriteString("\n### Findings excerpt\n")
		user.WriteString("```markdown\n")
		user.WriteString(findingsText)
		if !strings.HasSuffix(findingsText, "\n") {
			user.WriteString("\n")
		}
		user.WriteString("```\n")
	}
	if strings.TrimSpace(traceTail) != "" {
		user.WriteString("\n### Trace tail excerpt\n")
		user.WriteString("```text\n")
		user.WriteString(traceTail)
		if !strings.HasSuffix(traceTail, "\n") {
			user.WriteString("\n")
		}
		user.WriteString("```\n")
	}
	if strings.TrimSpace(testReportText) != "" {
		user.WriteString("\n### Test report excerpt\n")
		user.WriteString("```markdown\n")
		user.WriteString(testReportText)
		if !strings.HasSuffix(testReportText, "\n") {
			user.WriteString("\n")
		}
		user.WriteString("```\n")
	}

	messages := []llm.ChatMessage{
		llm.BuildSystemMessage(system),
		llm.BuildUserMessage(user.String()),
	}

	var temp float64 = 0
	maxTokens := 600
	opts := &llm.ChatCompletionOptions{
		Model:       strings.TrimSpace(o.Model),
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}

	out, err := o.Client.ChatCompletion(ctx, messages, opts)
	if err != nil {
		return ObserverDecision{}, err
	}

	decision, err := parseObserverDecision(out)
	if err != nil {
		return ObserverDecision{}, err
	}
	return decision, nil
}

func parseObserverDecision(out string) (ObserverDecision, error) {
	raw := strings.TrimSpace(out)
	var d ObserverDecision
	if err := json.Unmarshal([]byte(raw), &d); err == nil {
		return d, nil
	}

	if candidate := extractFirstJSONObject(raw); candidate != "" {
		if err := json.Unmarshal([]byte(candidate), &d); err == nil {
			return d, nil
		}
	}

	if xmlBlock := extractFirstXMLBlock(raw, "observer_decision"); xmlBlock != "" {
		if parsed, ok := parseObserverDecisionFromXML(xmlBlock); ok {
			return parsed, nil
		}
	}

	if fallback, ok := parseObserverDecisionFromText(raw); ok {
		return fallback, nil
	}

	return ObserverDecision{}, fmt.Errorf("invalid observer output (expected XML or JSON): %q", truncateString(raw, 300))
}

func extractFirstJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	if start < 0 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(raw); i++ {
		ch := raw[i]

		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(raw[start : i+1])
			}
			if depth < 0 {
				return ""
			}
		}
	}

	return ""
}

func extractFirstXMLBlock(raw string, tag string) string {
	raw = strings.TrimSpace(raw)
	tag = strings.TrimSpace(tag)
	if raw == "" || tag == "" {
		return ""
	}
	re := regexp.MustCompile(`(?is)<` + regexp.QuoteMeta(tag) + `\b[^>]*>.*?</` + regexp.QuoteMeta(tag) + `>`)
	return strings.TrimSpace(re.FindString(raw))
}

type observerDecisionXML struct {
	Pass            string   `xml:"pass"`
	Reason          string   `xml:"reason"`
	Evidence        []string `xml:"evidence>item"`
	NextSteps       string   `xml:"next_steps"`
	QuestionsForUser []string `xml:"questions_for_user>item"`
}

func parseObserverDecisionFromXML(raw string) (ObserverDecision, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ObserverDecision{}, false
	}

	var x observerDecisionXML
	if err := xml.Unmarshal([]byte(raw), &x); err != nil {
		return ObserverDecision{}, false
	}

	passText := strings.TrimSpace(x.Pass)
	if passText == "" {
		return ObserverDecision{}, false
	}

	pass := false
	switch strings.ToLower(passText) {
	case "true", "1", "yes", "y", "是":
		pass = true
	case "false", "0", "no", "n", "否":
		pass = false
	default:
		return ObserverDecision{}, false
	}

	evidence := make([]string, 0, len(x.Evidence))
	for _, e := range x.Evidence {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		evidence = append(evidence, e)
	}

	qs := make([]string, 0, len(x.QuestionsForUser))
	for _, q := range x.QuestionsForUser {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		qs = append(qs, q)
	}

	return ObserverDecision{
		Pass:             pass,
		Reason:           strings.TrimSpace(x.Reason),
		Evidence:         evidence,
		NextSteps:        strings.TrimSpace(x.NextSteps),
		QuestionsForUser: qs,
	}, true
}

var reObserverPass = regexp.MustCompile(`(?im)\bpass\b\s*[:：]\s*(true|false)\b`)
var reMarkdownOrderedListPrefix = regexp.MustCompile(`^\s*\d+\s*[.)]\s+`)

func parseObserverDecisionFromText(raw string) (ObserverDecision, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ObserverDecision{}, false
	}

	match := reObserverPass.FindStringSubmatch(raw)
	if len(match) < 2 {
		return ObserverDecision{}, false
	}

	pass := strings.EqualFold(strings.TrimSpace(match[1]), "true")
	decision := ObserverDecision{Pass: pass}

	decision.Reason = extractMarkdownSection(raw, []string{"理由", "Reason", "原因"})
	decision.NextSteps = extractMarkdownSection(raw, []string{"下一步", "Next steps", "Next Steps", "next_steps", "NextSteps"})
	decision.Evidence = parseMarkdownList(extractMarkdownSection(raw, []string{"证据", "Evidence"}))
	decision.QuestionsForUser = parseMarkdownList(extractMarkdownSection(raw, []string{"需要你确认", "Questions", "questions_for_user"}))

	return decision, true
}

func extractMarkdownSection(raw string, headings []string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(headings) == 0 {
		return ""
	}

	lines := strings.Split(raw, "\n")
	start := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}

		head := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		head = strings.TrimSpace(strings.TrimSuffix(head, ":"))
		head = strings.TrimSpace(strings.TrimSuffix(head, "："))
		for _, h := range headings {
			h = strings.TrimSpace(h)
			if h == "" {
				continue
			}
			if strings.Contains(head, h) {
				start = i + 1
				break
			}
		}
		if start >= 0 {
			break
		}
	}

	if start < 0 || start >= len(lines) {
		return ""
	}

	end := len(lines)
	for j := start; j < len(lines); j++ {
		if strings.HasPrefix(strings.TrimSpace(lines[j]), "#") {
			end = j
			break
		}
	}

	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

func parseMarkdownList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var out []string
	for _, line := range strings.Split(raw, "\n") {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		// Basic bullet/numbered list support: "- x", "* x", "1. x", "1) x"
		if strings.HasPrefix(l, "-") || strings.HasPrefix(l, "*") {
			l = strings.TrimSpace(strings.TrimLeft(l, "-*"))
		} else {
			l = reMarkdownOrderedListPrefix.ReplaceAllString(l, "")
			l = strings.TrimSpace(l)
		}
		if l == "" {
			continue
		}
		out = append(out, l)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func readFileHead(path string, maxBytes int) (string, string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "missing"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "not_found"
		}
		return "", "error"
	}
	if maxBytes > 0 && len(data) > maxBytes {
		return string(data[:maxBytes]), fmt.Sprintf("exists,size=%d,head=%d", len(data), maxBytes)
	}
	return string(data), fmt.Sprintf("exists,size=%d", len(data))
}

func readFileTail(path string, maxBytes int) (string, string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "missing"
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "not_found"
		}
		return "", "error"
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", "error"
	}
	size := info.Size()
	if size <= 0 {
		return "", "empty"
	}
	if maxBytes <= 0 || int64(maxBytes) >= size {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", "error"
		}
		return string(data), fmt.Sprintf("exists,size=%d", size)
	}

	start := size - int64(maxBytes)
	if _, err := f.Seek(start, 0); err != nil {
		return "", "error"
	}
	buf := make([]byte, maxBytes)
	n, _ := f.Read(buf)
	return string(buf[:n]), fmt.Sprintf("exists,size=%d,tail=%d", size, maxBytes)
}

func truncateString(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}
