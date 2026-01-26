package skill

import (
	"strings"
	"unicode"
)

type Source string

const (
	SourceOneAgent Source = ".oneagent"
	SourceClaude   Source = ".claude"
	SourceCodex    Source = ".codex"
)

type Skill struct {
	ID          string   `json:"skill_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	Source      Source   `json:"source"`
	Path        string   `json:"path"`
}

type Catalog struct {
	Skills []Skill

	byID   map[string]Skill
	byName map[string]Skill
	byPath map[string]Skill
}

func (c *Catalog) ByID(id string) (Skill, bool) {
	if c == nil || c.byID == nil {
		return Skill{}, false
	}
	s, ok := c.byID[NormalizeName(id)]
	return s, ok
}

func (c *Catalog) ByName(name string) (Skill, bool) {
	if c == nil || c.byName == nil {
		return Skill{}, false
	}
	s, ok := c.byName[NormalizeName(name)]
	return s, ok
}

func (c *Catalog) ByPath(path string) (Skill, bool) {
	if c == nil || c.byPath == nil {
		return Skill{}, false
	}
	s, ok := c.byPath[path]
	return s, ok
}

func NormalizeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(name))

	prevSep := false
	for _, r := range name {
		switch {
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if !prevSep {
				b.WriteByte('-')
				prevSep = true
			}
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevSep = false
		default:
			// Skip other punctuation to keep IDs stable and comparable.
			if !prevSep {
				b.WriteByte('-')
				prevSep = true
			}
		}
	}

	out := strings.Trim(b.String(), "-")
	return out
}

