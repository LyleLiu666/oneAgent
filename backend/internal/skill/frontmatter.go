package skill

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type frontmatter struct {
	Name        string                   `yaml:"name"`
	Description string                   `yaml:"description"`
	Tags        stringList               `yaml:"tags"`
	Keywords    stringList               `yaml:"keywords"`
	ToolIDs     stringList               `yaml:"tool_ids"`
	Requires    *requiresFrontmatter     `yaml:"requires"`
	Install     []installSpecFrontmatter `yaml:"install"`
}

type stringList []string

func (s *stringList) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case yaml.ScalarNode:
		v := strings.TrimSpace(value.Value)
		if v == "" {
			return nil
		}
		*s = []string{v}
		return nil
	case yaml.SequenceNode:
		out := make([]string, 0, len(value.Content))
		for _, n := range value.Content {
			if n == nil || n.Kind != yaml.ScalarNode {
				continue
			}
			v := strings.TrimSpace(n.Value)
			if v == "" {
				continue
			}
			out = append(out, v)
		}
		*s = out
		return nil
	default:
		return fmt.Errorf("unsupported yaml node kind: %d", value.Kind)
	}
}

func parseFrontmatter(markdown []byte) (frontmatter, []byte, bool) {
	const sep = "---"
	data := bytes.TrimPrefix(markdown, []byte{0xef, 0xbb, 0xbf}) // UTF-8 BOM
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) < 3 {
		return frontmatter{}, markdown, false
	}

	if strings.TrimSpace(string(lines[0])) != sep {
		return frontmatter{}, markdown, false
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(string(lines[i])) == sep {
			end = i
			break
		}
	}
	if end == -1 {
		return frontmatter{}, markdown, false
	}

	yamlBlock := bytes.Join(lines[1:end], []byte("\n"))
	var fm frontmatter
	if err := yaml.Unmarshal(yamlBlock, &fm); err != nil {
		return frontmatter{}, markdown, false
	}

	rest := bytes.Join(lines[end+1:], []byte("\n"))
	rest = bytes.TrimLeft(rest, "\r\n")
	return fm, rest, true
}

type requiresFrontmatter struct {
	OS      stringList `yaml:"os"`
	Bins    stringList `yaml:"bins"`
	AnyBins stringList `yaml:"any_bins"`
	Env     stringList `yaml:"env"`
}

type installSpecFrontmatter struct {
	Kind    string     `yaml:"kind"`
	Label   string     `yaml:"label"`
	Formula string     `yaml:"formula"`
	Module  string     `yaml:"module"`
	Package string     `yaml:"package"`
	URL     string     `yaml:"url"`
	Command string     `yaml:"command"`
	Bins    stringList `yaml:"bins"`
	OS      stringList `yaml:"os"`
}

func firstParagraph(text string, maxRunes int) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "\n\n")
	para := ""
	for _, part := range parts {
		candidate := strings.TrimSpace(part)
		if candidate == "" {
			continue
		}
		if strings.HasPrefix(candidate, "#") {
			continue
		}
		if strings.HasPrefix(candidate, ">") {
			continue
		}
		if strings.HasPrefix(candidate, "```") {
			continue
		}
		para = candidate
		break
	}
	if para == "" {
		return ""
	}

	runes := []rune(para)
	if maxRunes > 0 && len(runes) > maxRunes {
		para = string(runes[:maxRunes])
	}

	para = strings.ReplaceAll(para, "\n", " ")
	para = strings.Join(strings.Fields(para), " ")
	return para
}
