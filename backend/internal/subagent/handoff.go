package subagent

import (
	"html"
	"strings"
)

func parseHandoffXML(text string) (summary, timeline, findings, changedFiles string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", "", ""
	}

	raw, ok := extractLatestTagBlock(text, "subagent_handoff")
	if !ok {
		return "", "", "", ""
	}

	stopTags := []string{"summary", "timeline", "findings", "changed_files"}

	summary, _ = extractTagValue(raw, "summary", stopTags)
	timeline, _ = extractTagValue(raw, "timeline", stopTags)
	findings, _ = extractTagValue(raw, "findings", stopTags)
	changedFiles, _ = extractTagValue(raw, "changed_files", stopTags)

	return summary, timeline, findings, changedFiles
}

func buildFindingsMarkdown(timeline, findings, changedFiles string, changedList []string, planDone []PlanMarkDoneRecord) string {
	var b strings.Builder
	b.WriteString("# FINDINGS\n\n")

	writeSection(&b, "## 流水账", timeline, "## 流水账")
	b.WriteString("\n\n")
	writeSection(&b, "## Findings", findings, "## Findings")

	b.WriteString("\n\n")
	if strings.TrimSpace(changedFiles) != "" {
		writeSection(&b, "## 变更文件", changedFiles, "## 变更文件")
	} else {
		b.WriteString("## 变更文件\n")
		if len(changedList) == 0 {
			b.WriteString("- （无）\n")
		} else {
			for _, p := range changedList {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				b.WriteString("- ")
				b.WriteString(p)
				b.WriteString("\n")
			}
		}
	}

	if len(planDone) > 0 {
		b.WriteString("\n## Plan Mark Done\n")
		for _, rec := range planDone {
			line := "- task_id=" + strings.TrimSpace(rec.TaskID) + " pass="
			if rec.Pass {
				line += "true"
			} else {
				line += "false"
			}
			if strings.TrimSpace(rec.Reason) != "" {
				line += " reason=" + strings.TrimSpace(rec.Reason)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	return b.String()
}

func writeSection(b *strings.Builder, defaultHeader string, content string, requiredHeader string) {
	content = strings.TrimSpace(content)
	if content == "" {
		b.WriteString(defaultHeader)
		b.WriteString("\n- （无）")
		return
	}
	if requiredHeader != "" && !strings.Contains(content, requiredHeader) {
		b.WriteString(defaultHeader)
		b.WriteString("\n")
		b.WriteString(content)
		return
	}
	b.WriteString(content)
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
