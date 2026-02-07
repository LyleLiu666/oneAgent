package xmlprotocol

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

type recoveredWriteFileAppend struct {
	FilePath                 string
	Content                  string
	RepairedAssistantContent string
}

func recoverTruncatedWriteFileAppend(cleaned string) (recoveredWriteFileAppend, bool) {
	lower := strings.ToLower(cleaned)
	start := strings.LastIndex(lower, toolDataStart)
	if start == -1 {
		return recoveredWriteFileAppend{}, false
	}
	tail := cleaned[start:]

	toolName, ok := firstTagValueLoose(tail, []string{"tool_name", "toolName", "tool", "name"})
	if !ok {
		return recoveredWriteFileAppend{}, false
	}
	if canonicalToolName(toolName) != "write_file" {
		return recoveredWriteFileAppend{}, false
	}

	appendValue, ok := firstTagValue(tail, []string{"append"})
	if !ok || !parseBool(appendValue) {
		return recoveredWriteFileAppend{}, false
	}

	filePath, ok := firstTagValueLoose(tail, []string{"filePath", "file_path"})
	if !ok {
		return recoveredWriteFileAppend{}, false
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return recoveredWriteFileAppend{}, false
	}

	content, ok := tagValueAllowUnclosed(tail, "content", []string{"content", "call", "tool_data"})
	if !ok {
		return recoveredWriteFileAppend{}, false
	}

	repaired := strings.TrimSpace(cleaned[:start] + repairTruncatedToolDataTail(tail))
	if repaired == "" {
		repaired = strings.TrimSpace(cleaned)
	}

	return recoveredWriteFileAppend{
		FilePath:                 filePath,
		Content:                  content,
		RepairedAssistantContent: repaired,
	}, true
}

func repairTruncatedToolDataTail(tail string) string {
	repaired := tail

	// Close the content tag if it was started but not closed.
	if indexCaseInsensitive(repaired, "<content") != -1 && indexCaseInsensitive(repaired, "</content>") == -1 {
		repaired += "\n</content>"
	}
	// Close call/tool_data tags if missing.
	if indexCaseInsensitive(repaired, "<call") != -1 && indexCaseInsensitive(repaired, "</call>") == -1 {
		repaired += "\n  </call>"
	}
	if indexCaseInsensitive(repaired, "</tool_data>") == -1 {
		repaired += "\n</tool_data>"
	}
	return repaired
}

func tagValueAllowUnclosed(input, openTag string, closeTags []string) (string, bool) {
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

	end := len(rest)
	if minEnd >= 0 {
		end = minEnd
	}

	value := strings.TrimSpace(rest[:end])
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

func canonicalToolName(name string) string {
	out := strings.ToLower(strings.TrimSpace(name))
	out = strings.ReplaceAll(out, "-", "_")
	return out
}

func parseBool(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	b, err := strconv.ParseBool(trimmed)
	return err == nil && b
}
