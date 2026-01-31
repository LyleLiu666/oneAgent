package plan

import (
	"regexp"
	"strings"
)

var taskLineRe = regexp.MustCompile(`^\s*-\s*\[([xX>\s])\]\s*(.*?)(?:\s*<!--\s*id:\s*([^>]+?)\s*-->)\s*$`)

func Parse(content string) Plan {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	tasks := make([]Task, 0, 16)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		match := taskLineRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		mark := strings.TrimSpace(match[1])
		title := strings.TrimSpace(match[2])
		id := strings.TrimSpace(match[3])
		if id == "" {
			continue
		}

		task := Task{
			ID:        id,
			Title:     title,
			Status:    "todo",
			LineIndex: i,
		}
		if strings.EqualFold(mark, "x") {
			task.Status = "done"
		} else if mark == ">" {
			task.Status = "doing"
		}

		section := ""
		acceptKey := ""

		for j := i + 1; j < len(lines); j++ {
			next := lines[j]
			if taskLineRe.MatchString(next) {
				break
			}

			trimmed := strings.TrimSpace(next)
			if trimmed == "" {
				continue
			}

			switch trimmed {
			case "- scope:":
				section = "scope"
				acceptKey = ""
				continue
			case "- acceptance:":
				section = "acceptance"
				acceptKey = ""
				continue
			}

			if section == "scope" {
				if strings.HasPrefix(trimmed, "- ") {
					pat := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
					if pat != "" {
						task.Scope = append(task.Scope, pat)
					}
				}
				continue
			}

			if section == "acceptance" {
				switch trimmed {
				case "- files:":
					acceptKey = "files"
					continue
				case "- must_contain:":
					acceptKey = "must_contain"
					continue
				}

				if !strings.HasPrefix(trimmed, "- ") {
					continue
				}
				item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
				if item == "" {
					continue
				}

				switch acceptKey {
				case "files":
					task.Acceptance.Files = append(task.Acceptance.Files, item)
				case "must_contain":
					if mc, ok := parseMustContainItem(item); ok {
						task.Acceptance.MustContain = append(task.Acceptance.MustContain, mc)
					}
				}
			}
		}

		tasks = append(tasks, task)
	}

	return Plan{Tasks: tasks}
}

func parseMustContainItem(raw string) (MustContain, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return MustContain{}, false
	}
	colon := strings.Index(raw, ":")
	if colon <= 0 {
		return MustContain{}, false
	}
	path := strings.TrimSpace(raw[:colon])
	value := strings.TrimSpace(raw[colon+1:])
	value = strings.TrimPrefix(value, " ")
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}
	path = strings.TrimSpace(path)
	value = strings.TrimSpace(value)
	if path == "" || value == "" {
		return MustContain{}, false
	}
	return MustContain{Path: path, Text: value}, true
}

func findTaskByID(p Plan, id string) (Task, bool) {
	for _, t := range p.Tasks {
		if strings.TrimSpace(t.ID) == id {
			return t, true
		}
	}
	return Task{}, false
}
