package sopskill

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/fsutil"
	"github.com/liu_y/oneAgent/backend/internal/skill"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

type MaterializedSkill struct {
	SkillID   string
	SkillName string
	SkillPath string
}

func MaterializeSuggestion(home string, sug workledger.Suggestion) (MaterializedSkill, error) {
	home = strings.TrimSpace(home)
	if home == "" {
		return MaterializedSkill{}, errors.New("home is required")
	}

	title := strings.TrimSpace(sug.Title)
	if title == "" {
		title = "SOP"
	}
	suffix := strings.TrimSpace(sug.SuggestionID)
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	if suffix == "" {
		return MaterializedSkill{}, errors.New("suggestion_id is required")
	}

	name := fmt.Sprintf("%s %s", title, suffix)
	id := skill.NormalizeName(name)
	if id == "" {
		return MaterializedSkill{}, errors.New("invalid skill name")
	}

	desc := strings.TrimSpace(sug.Description)
	if desc == "" {
		desc = "Materialized SOP from work ledger"
	}

	body := strings.TrimSpace(sug.DraftSkill)
	if body == "" {
		return MaterializedSkill{}, errors.New("draft_skill is required")
	}

	var b strings.Builder
	b.Grow(len(body) + 512)
	b.WriteString("---\n")
	b.WriteString("name: ")
	b.WriteString(escapeYAMLScalar(name))
	b.WriteString("\n")
	b.WriteString("description: ")
	b.WriteString(escapeYAMLScalar(desc))
	b.WriteString("\n")
	b.WriteString("tags:\n")
	b.WriteString("  - sop\n")
	b.WriteString("  - auto-learned\n")
	b.WriteString("---\n\n")
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString("## Evidence\n")
	b.WriteString("- suggestion_id: ")
	b.WriteString(sug.SuggestionID)
	b.WriteString("\n")
	if len(sug.EvidenceReceiptIDs) > 0 {
		b.WriteString("- evidence_receipt_ids:\n")
		for _, id := range sug.EvidenceReceiptIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			b.WriteString("  - ")
			b.WriteString(id)
			b.WriteString("\n")
		}
	}

	skillPath := filepath.Join(home, ".oneagent", "skills", id, "SKILL.md")
	if err := fsutil.AtomicWriteFile(skillPath, []byte(b.String()), 0o644); err != nil {
		return MaterializedSkill{}, err
	}

	return MaterializedSkill{
		SkillID:   id,
		SkillName: name,
		SkillPath: skillPath,
	}, nil
}

func escapeYAMLScalar(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, `:#[]{}&,*!|>'"%@\\`) || strings.HasPrefix(s, "-") || strings.Contains(s, "\t") {
		escaped := strings.ReplaceAll(s, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return s
}
