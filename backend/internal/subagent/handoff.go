package subagent

import (
	"encoding/xml"
	"strings"
)

type handoffXML struct {
	Summary       string `xml:"summary"`
	Timeline      string `xml:"timeline"`
	Findings      string `xml:"findings"`
	ChangedFiles  string `xml:"changed_files"`
}

func parseHandoffXML(text string) (summary, timeline, findings, changedFiles string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", "", ""
	}

	start := strings.Index(text, "<subagent_handoff")
	if start == -1 {
		return "", "", "", ""
	}
	end := strings.Index(text[start:], "</subagent_handoff>")
	if end == -1 {
		return "", "", "", ""
	}
	end = start + end + len("</subagent_handoff>")

	raw := text[start:end]
	var parsed handoffXML
	if err := xml.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", "", "", ""
	}

	return strings.TrimSpace(parsed.Summary),
		strings.TrimSpace(parsed.Timeline),
		strings.TrimSpace(parsed.Findings),
		strings.TrimSpace(parsed.ChangedFiles)
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

