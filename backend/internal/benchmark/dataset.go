package benchmark

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Head2HeadDataset struct {
	ID          string          `json:"id"`
	Version     int             `json:"version"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Cases       []Head2HeadCase `json:"cases"`
}

type Head2HeadCase struct {
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Labels          []string          `json:"labels,omitempty"`
	Prompt          string            `json:"prompt"`
	DeliverableType string            `json:"deliverable_type"`
	Workspace       Head2HeadWorkspace `json:"workspace"`
	Acceptance      []AcceptanceRule  `json:"acceptance,omitempty"`
	EvidenceRequired []string         `json:"evidence_required,omitempty"`
	Mock            MockSpec          `json:"mock"`
}

type Head2HeadWorkspace struct {
	GitInit   bool       `json:"git_init,omitempty"`
	SeedFiles []SeedFile `json:"seed_files,omitempty"`
}

type SeedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type AcceptanceRule struct {
	Type     string `json:"type"`
	Path     string `json:"path,omitempty"`
	Contains string `json:"contains,omitempty"`
	Equals   string `json:"equals,omitempty"`
}

type MockSpec struct {
	Attempts []MockAttempt `json:"attempts,omitempty"`
}

type MockAttempt struct {
	Responses []MockResponse `json:"responses"`
}

type MockResponse struct {
	ToolCalls []MockToolCall `json:"tool_calls,omitempty"`
	Text      string         `json:"text,omitempty"`
}

type MockToolCall struct {
	CallID    string          `json:"call_id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func LoadHead2HeadDataset(path string) (Head2HeadDataset, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Head2HeadDataset{}, errors.New("dataset path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Head2HeadDataset{}, err
	}
	var ds Head2HeadDataset
	if err := json.Unmarshal(data, &ds); err != nil {
		return Head2HeadDataset{}, err
	}
	if err := ds.Validate(); err != nil {
		return Head2HeadDataset{}, fmt.Errorf("invalid dataset: %w", err)
	}
	return ds, nil
}

func (d Head2HeadDataset) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return errors.New("id is required")
	}
	if d.Version <= 0 {
		return errors.New("version must be > 0")
	}
	if strings.TrimSpace(d.Title) == "" {
		return errors.New("title is required")
	}
	if len(d.Cases) == 0 {
		return errors.New("cases is required")
	}

	seenIDs := make(map[string]struct{}, len(d.Cases))
	for i := range d.Cases {
		c := d.Cases[i]
		if strings.TrimSpace(c.ID) == "" {
			return fmt.Errorf("cases[%d].id is required", i)
		}
		if _, ok := seenIDs[c.ID]; ok {
			return fmt.Errorf("duplicate case id: %s", c.ID)
		}
		seenIDs[c.ID] = struct{}{}

		if strings.TrimSpace(c.Title) == "" {
			return fmt.Errorf("cases[%d].title is required", i)
		}
		if strings.TrimSpace(c.Prompt) == "" {
			return fmt.Errorf("cases[%d].prompt is required", i)
		}
		if strings.TrimSpace(c.DeliverableType) == "" {
			return fmt.Errorf("cases[%d].deliverable_type is required", i)
		}

		if len(c.Labels) == 0 {
			return fmt.Errorf("cases[%d].labels is required", i)
		}
		for _, l := range c.Labels {
			if strings.TrimSpace(l) == "" {
				return fmt.Errorf("cases[%d].labels contains empty value", i)
			}
		}

		for j := range c.Workspace.SeedFiles {
			sf := c.Workspace.SeedFiles[j]
			if strings.TrimSpace(sf.Path) == "" {
				return fmt.Errorf("cases[%d].workspace.seed_files[%d].path is required", i, j)
			}
			clean := filepath.ToSlash(filepath.Clean(sf.Path))
			if clean == "." || clean == "" {
				return fmt.Errorf("cases[%d].workspace.seed_files[%d].path is invalid", i, j)
			}
			if strings.HasPrefix(clean, "../") || clean == ".." {
				return fmt.Errorf("cases[%d].workspace.seed_files[%d].path must not escape workspace: %s", i, j, sf.Path)
			}
			if filepath.IsAbs(sf.Path) {
				return fmt.Errorf("cases[%d].workspace.seed_files[%d].path must be relative", i, j)
			}
		}

		for j := range c.Acceptance {
			r := c.Acceptance[j]
			typ := strings.TrimSpace(r.Type)
			if typ == "" {
				return fmt.Errorf("cases[%d].acceptance[%d].type is required", i, j)
			}
			switch typ {
			case "file_exists", "file_not_exists", "json_valid", "changed_files_contains":
				if strings.TrimSpace(r.Path) == "" {
					return fmt.Errorf("cases[%d].acceptance[%d].path is required for type=%s", i, j, typ)
				}
			case "file_contains", "file_not_contains":
				if strings.TrimSpace(r.Path) == "" || r.Contains == "" {
					return fmt.Errorf("cases[%d].acceptance[%d].path+contains is required for type=%s", i, j, typ)
				}
			case "file_equals":
				if strings.TrimSpace(r.Path) == "" {
					return fmt.Errorf("cases[%d].acceptance[%d].path is required for type=%s", i, j, typ)
				}
			default:
				return fmt.Errorf("cases[%d].acceptance[%d].type is unknown: %s", i, j, typ)
			}
		}

		if len(c.Mock.Attempts) == 0 {
			return fmt.Errorf("cases[%d].mock.attempts is required", i)
		}
		for ai := range c.Mock.Attempts {
			attempt := c.Mock.Attempts[ai]
			if len(attempt.Responses) == 0 {
				return fmt.Errorf("cases[%d].mock.attempts[%d].responses is required", i, ai)
			}
			for ri := range attempt.Responses {
				resp := attempt.Responses[ri]
				if len(resp.ToolCalls) == 0 && strings.TrimSpace(resp.Text) == "" {
					return fmt.Errorf("cases[%d].mock.attempts[%d].responses[%d] must have tool_calls or text", i, ai, ri)
				}
				for ti := range resp.ToolCalls {
					tc := resp.ToolCalls[ti]
					if strings.TrimSpace(tc.CallID) == "" {
						return fmt.Errorf("cases[%d].mock.attempts[%d].responses[%d].tool_calls[%d].call_id is required", i, ai, ri, ti)
					}
					if strings.TrimSpace(tc.Name) == "" {
						return fmt.Errorf("cases[%d].mock.attempts[%d].responses[%d].tool_calls[%d].name is required", i, ai, ri, ti)
					}
					if len(tc.Arguments) == 0 || !json.Valid(tc.Arguments) {
						return fmt.Errorf("cases[%d].mock.attempts[%d].responses[%d].tool_calls[%d].arguments must be valid JSON", i, ai, ri, ti)
					}
				}
			}
			// To complete the tool loop, at least one response must have no tool calls.
			sawTerminal := false
			for _, r := range attempt.Responses {
				if len(r.ToolCalls) == 0 {
					sawTerminal = true
					break
				}
			}
			if !sawTerminal {
				return fmt.Errorf("cases[%d].mock.attempts[%d] has no terminal response (no tool_calls)", i, ai)
			}
		}
	}

	return nil
}
