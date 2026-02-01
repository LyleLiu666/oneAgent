package toolxml

import (
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
)

type Call struct {
	ToolName string
	Fields   map[string]string
	Raw      string
}

var (
	reThinking = regexp.MustCompile(`(?is)<thinking\b[^>]*>.*?</thinking>`)
	reThink    = regexp.MustCompile(`(?is)<think\b[^>]*>.*?</think>`)
	reCall     = regexp.MustCompile(`(?is)<call\b[^>]*>.*?</call>`)
)

// StripThinking removes <thinking>...</thinking> and <think>...</think> blocks.
func StripThinking(input string) string {
	if input == "" {
		return ""
	}
	out := reThinking.ReplaceAllString(input, "")
	out = reThink.ReplaceAllString(out, "")
	return out
}

// ExtractLatestToolData finds the last <tool_data>...</tool_data> block after stripping thinking sections.
func ExtractLatestToolData(input string) (string, bool) {
	cleaned := StripThinking(input)
	lower := strings.ToLower(cleaned)

	start := strings.LastIndex(lower, "<tool_data")
	if start == -1 {
		return "", false
	}

	startTagEndRel := strings.Index(cleaned[start:], ">")
	if startTagEndRel == -1 {
		return "", false
	}
	startTagEnd := start + startTagEndRel + 1

	endRel := strings.Index(lower[startTagEnd:], "</tool_data>")
	if endRel == -1 {
		return "", false
	}
	end := startTagEnd + endRel + len("</tool_data>")
	return cleaned[start:end], true
}

func ParseToolData(toolDataBlock string) ([]Call, error) {
	trimmed := strings.TrimSpace(toolDataBlock)
	if trimmed == "" {
		return nil, errors.New("empty tool_data")
	}

	lower := strings.ToLower(trimmed)
	start := strings.Index(lower, "<tool_data")
	if start == -1 {
		return nil, errors.New("missing <tool_data>")
	}
	startTagEndRel := strings.Index(trimmed[start:], ">")
	if startTagEndRel == -1 {
		return nil, errors.New("malformed <tool_data> start tag")
	}
	startTagEnd := start + startTagEndRel + 1
	end := strings.LastIndex(lower, "</tool_data>")
	if end == -1 || end < startTagEnd {
		return nil, errors.New("missing </tool_data>")
	}

	inner := trimmed[startTagEnd:end]
	calls := make([]Call, 0)

	rawCalls := reCall.FindAllString(inner, -1)
	if len(rawCalls) == 0 {
		call, err := parseCall(inner, inner)
		if err != nil {
			return nil, err
		}
		return []Call{call}, nil
	}

	for _, rawCall := range rawCalls {
		callInner, ok := extractInnerXML(rawCall, "call")
		if !ok {
			continue
		}
		call, err := parseCall(callInner, rawCall)
		if err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}

	if len(calls) == 0 {
		return nil, errors.New("no valid <call> blocks found")
	}
	return calls, nil
}

func parseCall(callInner string, raw string) (Call, error) {
	callInner = repairMissingFilePathOpenTag(callInner)

	fields := map[string]string{}

	toolName, ok := firstTagValueLoose(callInner, []string{"tool_name", "toolName", "tool", "name"})
	if !ok {
		return Call{}, errors.New("missing <tool_name>")
	}
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return Call{}, errors.New("empty <tool_name>")
	}

	for _, tag := range []string{
		"command",
		"timeout_ms",
		"action",
		"job_id",
		"jobId",
		"wait_seconds",
		"waitSeconds",
		"wait_ms",
		"waitMs",
		"max_runtime_seconds",
		"maxRuntimeSeconds",
		"max_runtime_ms",
		"maxRuntimeMs",
		"max_log_bytes",
		"maxLogBytes",
		"stdout_offset",
		"stdoutOffset",
		"stderr_offset",
		"stderrOffset",
		"max_delta_bytes",
		"maxDeltaBytes",
		"task",
		"context_summary",
		"contextSummary",
		"tool_ids",
		"toolIds",
		"scope",
		"max_steps",
		"maxSteps",
		"k_skills",
		"kSkills",
		"skill_ids",
		"skillIds",
		"filePath",
		"file_path",
		"offset_lines",
		"offsetLines",
		"limit_lines",
		"limitLines",
		"max_bytes",
		"maxBytes",
		"oldcontent",
		"newcontent",
		"replaceAll",
		"replace_all",
		"content",
		"append",
		"pattern",
		"path",
		"query",
		"count",
		"freshness",
		"max_results",
		"maxResults",
		"fixed_strings",
		"fixedStrings",
		"task_id",
		"taskId",
		"template",
		"overwrite",
		"name",
		"skill_id",
		"id",
		"edits",
	} {
		if value, ok := tagValue(callInner, tag); ok {
			fields[tag] = value
		}
	}

	// Normalize common aliases.
	if _, ok := fields["filePath"]; !ok {
		if v, ok := fields["file_path"]; ok {
			fields["filePath"] = v
		}
	}
	if _, ok := fields["replaceAll"]; !ok {
		if v, ok := fields["replace_all"]; ok {
			fields["replaceAll"] = v
		}
	}
	if _, ok := fields["job_id"]; !ok {
		if v, ok := fields["jobId"]; ok {
			fields["job_id"] = v
		}
	}
	if _, ok := fields["wait_ms"]; !ok {
		if v, ok := fields["waitMs"]; ok {
			fields["wait_ms"] = v
		}
	}
	if _, ok := fields["wait_seconds"]; !ok {
		if v, ok := fields["waitSeconds"]; ok {
			fields["wait_seconds"] = v
		}
	}
	if _, ok := fields["max_runtime_ms"]; !ok {
		if v, ok := fields["maxRuntimeMs"]; ok {
			fields["max_runtime_ms"] = v
		}
	}
	if _, ok := fields["max_runtime_seconds"]; !ok {
		if v, ok := fields["maxRuntimeSeconds"]; ok {
			fields["max_runtime_seconds"] = v
		}
	}
	if _, ok := fields["max_log_bytes"]; !ok {
		if v, ok := fields["maxLogBytes"]; ok {
			fields["max_log_bytes"] = v
		}
	}
	if _, ok := fields["stdout_offset"]; !ok {
		if v, ok := fields["stdoutOffset"]; ok {
			fields["stdout_offset"] = v
		}
	}
	if _, ok := fields["stderr_offset"]; !ok {
		if v, ok := fields["stderrOffset"]; ok {
			fields["stderr_offset"] = v
		}
	}
	if _, ok := fields["max_delta_bytes"]; !ok {
		if v, ok := fields["maxDeltaBytes"]; ok {
			fields["max_delta_bytes"] = v
		}
	}
	if _, ok := fields["max_results"]; !ok {
		if v, ok := fields["maxResults"]; ok {
			fields["max_results"] = v
		}
	}
	if _, ok := fields["fixed_strings"]; !ok {
		if v, ok := fields["fixedStrings"]; ok {
			fields["fixed_strings"] = v
		}
	}
	if _, ok := fields["context_summary"]; !ok {
		if v, ok := fields["contextSummary"]; ok {
			fields["context_summary"] = v
		}
	}
	if _, ok := fields["tool_ids"]; !ok {
		if v, ok := fields["toolIds"]; ok {
			fields["tool_ids"] = v
		}
	}
	if _, ok := fields["max_steps"]; !ok {
		if v, ok := fields["maxSteps"]; ok {
			fields["max_steps"] = v
		}
	}
	if _, ok := fields["k_skills"]; !ok {
		if v, ok := fields["kSkills"]; ok {
			fields["k_skills"] = v
		}
	}
	if _, ok := fields["skill_ids"]; !ok {
		if v, ok := fields["skillIds"]; ok {
			fields["skill_ids"] = v
		}
	}
	if _, ok := fields["task_id"]; !ok {
		if v, ok := fields["taskId"]; ok {
			fields["task_id"] = v
		}
	}
	if _, ok := fields["offset_lines"]; !ok {
		if v, ok := fields["offsetLines"]; ok {
			fields["offset_lines"] = v
		}
	}
	if _, ok := fields["limit_lines"]; !ok {
		if v, ok := fields["limitLines"]; ok {
			fields["limit_lines"] = v
		}
	}
	if _, ok := fields["max_bytes"]; !ok {
		if v, ok := fields["maxBytes"]; ok {
			fields["max_bytes"] = v
		}
	}

	return Call{
		ToolName: toolName,
		Fields:   fields,
		Raw:      raw,
	}, nil
}

func repairMissingFilePathOpenTag(input string) string {
	lower := strings.ToLower(input)
	if strings.Contains(lower, "<filepath") {
		return input
	}

	// Repair a common malformed pattern produced by LLMs:
	//   <tool_name>read_file</filePath>src/generator.py</filePath>
	// by fixing the wrong close tag and inserting the missing `<filePath>` opening tag.
	reToolNameClosedByFilePath := regexp.MustCompile(`(?is)<tool_name\b[^>]*>\s*([^<]+?)\s*</filepath>\s*([^<]+?)\s*</filepath>`)
	if reToolNameClosedByFilePath.MatchString(input) {
		return reToolNameClosedByFilePath.ReplaceAllString(input, `<tool_name>${1}</tool_name><filePath>${2}</filePath>`)
	}

	// Repair a common malformed pattern produced by LLMs:
	//   <tool_name>edit</toolName>/abs/path</filePath>
	// by inserting the missing `<filePath>` opening tag.
	re := regexp.MustCompile(`(?is)(</tool_name>|</toolname>)\s*([^<]+?)\s*(</filepath>)`)
	return re.ReplaceAllString(input, `${1}<filePath>${2}${3}`)
}

func firstTagValue(input string, tags []string) (string, bool) {
	for _, tag := range tags {
		if v, ok := tagValue(input, tag); ok {
			return v, true
		}
	}
	return "", false
}

func firstTagValueLoose(input string, tags []string) (string, bool) {
	for _, open := range tags {
		if v, ok := tagValueWithAnyClose(input, open, tags); ok {
			return v, true
		}
	}
	return "", false
}

func tagValue(input, tag string) (string, bool) {
	re, err := regexp.Compile(fmt.Sprintf(`(?is)<%s\b[^>]*>(.*?)</%s>`, regexp.QuoteMeta(tag), regexp.QuoteMeta(tag)))
	if err != nil {
		return "", false
	}
	match := re.FindStringSubmatch(input)
	if len(match) < 2 {
		return "", false
	}
	value := strings.TrimSpace(match[1])
	if value == "" {
		return "", true
	}

	// CDATA support.
	if strings.HasPrefix(value, "<![CDATA[") {
		if end := strings.Index(value, "]]>"); end != -1 {
			value = value[len("<![CDATA["):end]
		} else {
			// Be tolerant of malformed CDATA blocks so tool calls don't
			// accidentally pass a leading "<" into downstream tools.
			value = value[len("<![CDATA["):]
		}
	}

	value = html.UnescapeString(value)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return value, true
}

func tagValueWithAnyClose(input string, openTag string, closeTags []string) (string, bool) {
	openRe, err := regexp.Compile(fmt.Sprintf(`(?is)<%s\b[^>]*>`, regexp.QuoteMeta(openTag)))
	if err != nil {
		return "", false
	}
	loc := openRe.FindStringIndex(input)
	if len(loc) != 2 {
		return "", false
	}
	start := loc[1]

	rest := input[start:]
	minEnd := -1
	for _, closeTag := range closeTags {
		closeRe, err := regexp.Compile(fmt.Sprintf(`(?is)</%s>`, regexp.QuoteMeta(closeTag)))
		if err != nil {
			continue
		}
		if matchLoc := closeRe.FindStringIndex(rest); len(matchLoc) == 2 {
			if minEnd == -1 || matchLoc[0] < minEnd {
				minEnd = matchLoc[0]
			}
		}
	}
	if minEnd < 0 {
		return "", false
	}

	value := strings.TrimSpace(rest[:minEnd])
	if value == "" {
		return "", true
	}

	// CDATA support.
	if strings.HasPrefix(value, "<![CDATA[") {
		if end := strings.Index(value, "]]>"); end != -1 {
			value = value[len("<![CDATA["):end]
		} else {
			value = value[len("<![CDATA["):]
		}
	}

	value = html.UnescapeString(value)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return value, true
}

func extractInnerXML(block, tag string) (string, bool) {
	lower := strings.ToLower(block)
	start := strings.Index(lower, "<"+strings.ToLower(tag))
	if start == -1 {
		return "", false
	}
	startTagEndRel := strings.Index(block[start:], ">")
	if startTagEndRel == -1 {
		return "", false
	}
	startTagEnd := start + startTagEndRel + 1
	end := strings.LastIndex(lower, "</"+strings.ToLower(tag)+">")
	if end == -1 || end < startTagEnd {
		return "", false
	}
	return block[startTagEnd:end], true
}
