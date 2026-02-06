package taskqueue

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
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

	// MaxParseRetries controls how many times the observer will retry when the
	// model output cannot be parsed as a decision (best-effort self-heal).
	// Total attempts = 1 + MaxParseRetries.
	// When set to 0, it defaults to 1 (i.e., two total attempts). Negative
	// values disable retries.
	MaxParseRetries int
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
你是一个自主编码智能体的“结果观察员 (Outcome Observer)”。

【目标】
判断当前的尝试是否满足了用户的原始期望。

【核心评估逻辑：动态标准】
在评估之前，你必须先判断用户的任务类型，并采用不同的验收严格度：

1. **普通/查询/开放式任务**（如：查询天气、闲聊、通用问答、翻译）：
   - **标准**：只要核心信息准确、逻辑通顺且回答了用户的主要问题，即可判定为通过。
   - **宽容度**：**高**。不要因为缺少用户未明确要求的次要细节而判定失败（例如：用户问“今天天气如何”，只要回答了阴晴和温度即可，不能因为缺少“风力数据”或“湿度”而判定失败）。

2. **工程/代码/精确任务**（如：写代码、Bug修复、数据提取、数学计算）：
   - **标准**：必须严格符合所有约束条件、代码可运行、测试通过、且逻辑无误。
   - **宽容度**：**低**。必须严格基于证据（文件路径、代码片段、测试日志）进行验收。

【规则】
- **基于证据**：优先引用提供的工件（Artifacts）中的文件路径和摘录。
- **实质重于形式**：不要仅依赖 TODO 列表或计划清单是否存在，重点看实际产出是否已完成。
- **只读模式**：你不能运行命令，除非提供了测试运行的日志证据，否则不能假设代码已成功运行。
- **失败判定**：
  - 如果是**工程任务**且证据不足，必须判定失败 (FAIL) 并解释缺失什么证据。
  - 如果是**普通任务**，只要回答合理，即使缺乏某些技术性证据（如API原始响应体），也可以判定通过。
- **下一步计划**：如果 pass=false，必须提出可执行的 next_steps 作为下一次尝试的行动计划。
- **自主决策**：对于合理的“后续问题”，应根据现有证据选择路径，而不是反问用户。

【输出格式】
仅返回一个 XML 代码块：

<observer_decision>
  <pass>true 或 false</pass>
  <reason>简要说明判定的理由（区分任务类型和验收标准）...</reason>
  <evidence>
    <item>引用的证据 1...</item>
    <item>引用的证据 2...</item>
  </evidence>
  <next_steps>如果是 false，这里填写具体的补救步骤...</next_steps>
  <questions_for_user>
    <item>仅在绝对必要时填写...</item>
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
	maxTokens := 1200
	opts := &llm.ChatCompletionOptions{
		Model:       strings.TrimSpace(o.Model),
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}

	maxParseRetries := o.MaxParseRetries
	if maxParseRetries == 0 {
		maxParseRetries = 1
	}
	if maxParseRetries < 0 {
		maxParseRetries = 0
	}

	// Prefer tool-calling structured output when supported by the client.
	if client, ok := o.Client.(interface {
		ChatCompletionWithTools(context.Context, []llm.ChatMessage, *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error)
	}); ok {
		toolSpec := llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "observer_decision",
				Description: "Return the observer decision (pass/reason/evidence/next_steps/questions_for_user).",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pass":   map[string]any{"type": "boolean"},
						"reason": map[string]any{"type": "string"},
						"evidence": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
						},
						"next_steps": map[string]any{"type": "string"},
						"questions_for_user": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
						},
					},
					"required": []string{"pass", "reason", "evidence", "next_steps", "questions_for_user"},
				},
			},
		}

		toolMsgs := append([]llm.ChatMessage(nil), messages...)
		toolMsgs = append(toolMsgs, llm.BuildUserMessage(strings.TrimSpace(`
请立刻调用 tool：observer_decision。
要求：
- 只通过 tool arguments 返回结构化字段，不要输出任何额外文本
- 字段必须齐全：pass/reason/evidence/next_steps/questions_for_user
`)))

		toolOpts := *opts
		toolOpts.Tools = []llm.Tool{toolSpec}

		result, err := client.ChatCompletionWithTools(ctx, toolMsgs, &toolOpts)
		if err == nil {
			if decision, ok := parseObserverDecisionFromToolCalls(result.ToolCalls); ok {
				return decision, nil
			}
		}
	}

	var lastErr error
	for attempt := 0; attempt <= maxParseRetries; attempt++ {
		out, err := o.Client.ChatCompletion(ctx, messages, opts)
		if err != nil {
			return ObserverDecision{}, err
		}

		decision, err := parseObserverDecision(out)
		if err == nil {
			return decision, nil
		}
		lastErr = err

		if attempt >= maxParseRetries {
			break
		}

		repair := strings.TrimSpace(`
Your previous output was invalid and could not be parsed.

Return ONLY a single XML block that matches exactly this schema (no code fences, no extra text):

<observer_decision>
  <pass>true|false</pass>
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
		repair += "\n\nInvalid output (truncated):\n" + truncateString(strings.TrimSpace(out), 300)
		messages = append(messages, llm.BuildUserMessage(repair))
	}

	if lastErr == nil {
		lastErr = errors.New("observer decision parse failed")
	}
	return ObserverDecision{}, lastErr
}

func parseObserverDecisionFromToolCalls(calls []llm.ToolCall) (ObserverDecision, bool) {
	if len(calls) == 0 {
		return ObserverDecision{}, false
	}

	for _, call := range calls {
		name := strings.TrimSpace(call.Function.Name)
		if name != "observer_decision" {
			continue
		}

		raw := strings.TrimSpace(call.Function.Arguments)
		if raw == "" {
			continue
		}

		var d ObserverDecision
		if err := json.Unmarshal([]byte(raw), &d); err == nil {
			if err := validateObserverDecision(d); err == nil {
				return d, true
			}
		}

		if candidate := extractFirstJSONObject(raw); candidate != "" {
			if err := json.Unmarshal([]byte(candidate), &d); err == nil {
				if err := validateObserverDecision(d); err == nil {
					return d, true
				}
			}
		}
	}
	return ObserverDecision{}, false
}

func parseObserverDecision(out string) (ObserverDecision, error) {
	raw := strings.TrimSpace(out)
	var d ObserverDecision
	var validationErr error

	if err := json.Unmarshal([]byte(raw), &d); err == nil {
		if err := validateObserverDecision(d); err == nil {
			return d, nil
		} else {
			validationErr = err
		}
	}

	if candidate := extractFirstJSONObject(raw); candidate != "" {
		if err := json.Unmarshal([]byte(candidate), &d); err == nil {
			if err := validateObserverDecision(d); err == nil {
				return d, nil
			} else {
				validationErr = err
			}
		}
	}

	if xmlBlock := extractFirstXMLBlock(raw, "observer_decision"); xmlBlock != "" {
		if parsed, ok := parseObserverDecisionFromXML(xmlBlock); ok {
			if err := validateObserverDecision(parsed); err == nil {
				return parsed, nil
			} else {
				validationErr = err
			}
		}
		if parsed, ok := parseObserverDecisionFromTags(xmlBlock); ok {
			if err := validateObserverDecision(parsed); err == nil {
				return parsed, nil
			} else {
				validationErr = err
			}
		}
	}

	if xmlBlock, ok := repairObserverDecisionXML(raw); ok {
		if parsed, ok := parseObserverDecisionFromXML(xmlBlock); ok {
			if err := validateObserverDecision(parsed); err == nil {
				return parsed, nil
			} else {
				validationErr = err
			}
		}
		if parsed, ok := parseObserverDecisionFromTags(xmlBlock); ok {
			if err := validateObserverDecision(parsed); err == nil {
				return parsed, nil
			} else {
				validationErr = err
			}
		}
	}

	if fallback, ok := parseObserverDecisionFromText(raw); ok {
		if err := validateObserverDecision(fallback); err == nil {
			return fallback, nil
		} else {
			validationErr = err
		}
	}

	if validationErr != nil {
		return ObserverDecision{}, validationErr
	}

	return ObserverDecision{}, fmt.Errorf("invalid observer output (expected XML or JSON): %q", truncateString(raw, 300))
}

func validateObserverDecision(d ObserverDecision) error {
	if d.Pass {
		return nil
	}
	if strings.TrimSpace(d.Reason) == "" {
		return errors.New("observer decision is missing required field: reason (pass=false)")
	}
	if strings.TrimSpace(d.NextSteps) == "" {
		return errors.New("observer decision is missing required field: next_steps (pass=false)")
	}
	return nil
}

func parseObserverPassValue(passText string) (bool, bool) {
	passText = strings.TrimSpace(passText)
	if passText == "" {
		return false, false
	}

	lower := strings.ToLower(strings.TrimSpace(passText))
	lower = strings.Trim(lower, "\"'`")
	lower = strings.Trim(lower, " \t\r\n.,;:!?，。；：！？")
	if lower == "" {
		return false, false
	}

	switch lower {
	case "true", "1", "yes", "y", "是", "pass", "passed", "ok", "success", "succeeded", "通过", "成功":
		return true, true
	case "false", "0", "no", "n", "否", "fail", "failed", "ng", "error", "不通过", "失败":
		return false, true
	}

	// Tolerate truncated values when the model output is cut off mid-token.
	if len(lower) >= 2 {
		if strings.HasPrefix("true", lower) {
			return true, true
		}
		if strings.HasPrefix("false", lower) {
			return false, true
		}
		if strings.HasPrefix("pass", lower) {
			return true, true
		}
		if strings.HasPrefix("fail", lower) {
			return false, true
		}
	}

	// Last resort: accept contains() to handle mild formatting noise (e.g. "true." or "fail (missing)").
	if strings.Contains(lower, "true") || strings.Contains(lower, "pass") || strings.Contains(lower, "通过") || strings.Contains(lower, "成功") {
		return true, true
	}
	if strings.Contains(lower, "false") || strings.Contains(lower, "fail") || strings.Contains(lower, "不通过") || strings.Contains(lower, "失败") {
		return false, true
	}

	return false, false
}

func repairObserverDecisionXML(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	lower := strings.ToLower(raw)
	openIdx := strings.Index(lower, "<observer_decision")
	if openIdx < 0 {
		return "", false
	}
	if strings.Contains(lower, "</observer_decision>") {
		return "", false
	}

	candidate := strings.TrimSpace(raw[openIdx:]) + "\n</observer_decision>"
	return candidate, true
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

func extractLatestTagBlock(text string, tag string) (string, bool) {
	text = strings.TrimSpace(text)
	tag = strings.ToLower(strings.TrimSpace(tag))
	if text == "" || tag == "" {
		return "", false
	}

	lower := strings.ToLower(text)
	closeTag := "</" + tag + ">"

	end := strings.LastIndex(lower, closeTag)
	if end == -1 {
		start := strings.LastIndex(lower, "<"+tag)
		if start == -1 {
			return "", false
		}
		return strings.TrimSpace(text[start:]), true
	}
	end = end + len(closeTag)

	start := strings.LastIndex(lower[:end], "<"+tag)
	if start == -1 {
		return "", false
	}
	return strings.TrimSpace(text[start:end]), true
}

func extractTagValue(block string, tag string, stopTags []string) (string, bool) {
	block = strings.TrimSpace(block)
	tag = strings.ToLower(strings.TrimSpace(tag))
	if block == "" || tag == "" {
		return "", false
	}

	lower := strings.ToLower(block)

	start := strings.Index(lower, "<"+tag)
	if start == -1 {
		return "", false
	}
	startTagEndRel := strings.Index(block[start:], ">")
	if startTagEndRel == -1 {
		return "", false
	}
	startTagEnd := start + startTagEndRel + 1

	endTag := "</" + tag + ">"
	endRel := strings.Index(lower[startTagEnd:], endTag)
	end := -1
	if endRel != -1 {
		end = startTagEnd + endRel
	} else if len(stopTags) > 0 {
		next := len(block)
		for _, stop := range stopTags {
			stop = strings.ToLower(strings.TrimSpace(stop))
			if stop == "" || stop == tag {
				continue
			}
			idx := strings.Index(lower[startTagEnd:], "<"+stop)
			if idx == -1 {
				continue
			}
			abs := startTagEnd + idx
			if abs < next {
				next = abs
			}
		}
		if next != len(block) {
			end = next
		}
	}

	if end == -1 {
		end = len(block)
	}
	if end < startTagEnd {
		return "", false
	}

	value := strings.TrimSpace(block[startTagEnd:end])
	if value == "" {
		return "", true
	}

	// Default-CDATA behavior: accept raw text, but also tolerate explicit/malformed CDATA.
	if strings.HasPrefix(value, "<![CDATA[") {
		if end := strings.Index(value, "]]>"); end != -1 {
			value = value[len("<![CDATA["):end]
		} else {
			value = value[len("<![CDATA["):]
		}
	}

	value = html.UnescapeString(value)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.TrimSpace(value), true
}

type observerDecisionXML struct {
	Pass             string   `xml:"pass"`
	Reason           string   `xml:"reason"`
	Evidence         []string `xml:"evidence>item"`
	NextSteps        string   `xml:"next_steps"`
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

	pass, ok := parseObserverPassValue(passText)
	if !ok {
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

func parseObserverDecisionFromTags(raw string) (ObserverDecision, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ObserverDecision{}, false
	}

	block, ok := extractLatestTagBlock(raw, "observer_decision")
	if !ok {
		block = raw
	}

	passText, ok := extractTagValue(block, "pass", []string{"reason", "evidence", "next_steps", "questions_for_user"})
	passText = strings.TrimSpace(passText)
	if !ok || passText == "" {
		return ObserverDecision{}, false
	}

	pass, ok := parseObserverPassValue(passText)
	if !ok {
		return ObserverDecision{}, false
	}

	reason, _ := extractTagValue(block, "reason", []string{"evidence", "next_steps", "questions_for_user"})
	evidenceRaw, _ := extractTagValue(block, "evidence", []string{"next_steps", "questions_for_user"})
	nextSteps, _ := extractTagValue(block, "next_steps", []string{"questions_for_user"})
	questionsRaw, _ := extractTagValue(block, "questions_for_user", nil)

	evidence := parseObserverDecisionItems(evidenceRaw)
	qs := parseObserverDecisionItems(questionsRaw)

	return ObserverDecision{
		Pass:             pass,
		Reason:           strings.TrimSpace(reason),
		Evidence:         evidence,
		NextSteps:        strings.TrimSpace(nextSteps),
		QuestionsForUser: qs,
	}, true
}

func parseObserverDecisionItems(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	items := extractAllTagBlocks(raw, "item")
	if len(items) == 0 {
		return nil
	}

	out := make([]string, 0, len(items))
	for _, item := range items {
		v, ok := extractTagValue(item, "item", nil)
		v = strings.TrimSpace(v)
		if !ok || v == "" {
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

var reObserverPass = regexp.MustCompile(`(?im)\bpass\b\s*[:：]\s*(true|false|pass|fail|passed|failed|yes|no)\b`)
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

	pass, ok := parseObserverPassValue(match[1])
	if !ok {
		return ObserverDecision{}, false
	}
	decision := ObserverDecision{Pass: pass}

	decision.Reason = extractMarkdownSection(raw, []string{"理由", "Reason", "原因"})
	decision.NextSteps = extractMarkdownSection(raw, []string{"下一步", "Next steps", "Next Steps", "next_steps", "NextSteps"})
	decision.Evidence = parseMarkdownList(extractMarkdownSection(raw, []string{"证据", "Evidence"}))
	decision.QuestionsForUser = parseMarkdownList(extractMarkdownSection(raw, []string{"需要你确认", "Questions", "questions_for_user"}))

	return decision, true
}

func extractAllTagBlocks(text string, tag string) []string {
	text = strings.TrimSpace(text)
	tag = strings.ToLower(strings.TrimSpace(tag))
	if text == "" || tag == "" {
		return nil
	}

	lower := strings.ToLower(text)
	closeTag := "</" + tag + ">"

	var out []string
	search := 0
	for {
		openRel := strings.Index(lower[search:], "<"+tag)
		if openRel == -1 {
			break
		}
		open := search + openRel
		closeRel := strings.Index(lower[open:], closeTag)
		if closeRel == -1 {
			out = append(out, strings.TrimSpace(text[open:]))
			break
		}
		end := open + closeRel + len(closeTag)
		out = append(out, strings.TrimSpace(text[open:end]))
		search = end
	}
	return out
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
