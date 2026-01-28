package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/lsp"
	"github.com/liu_y/oneAgent/backend/internal/scope"
)

type lspLanguage string

const (
	lspLangGo lspLanguage = "go"
	lspLangTS lspLanguage = "ts"
)

type lspServer struct {
	Command string
	Args    []string
	Name    string
}

func lspLanguageForPath(path string) (lspLanguage, error) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
	switch ext {
	case ".go":
		return lspLangGo, nil
	case ".ts", ".tsx", ".js", ".jsx":
		return lspLangTS, nil
	default:
		return "", fmt.Errorf("unsupported language for file extension: %s", ext)
	}
}

func lspServerForLanguage(lang lspLanguage) lspServer {
	switch lang {
	case lspLangGo:
		cmd := strings.TrimSpace(getEnv("ONEAGENT_LSP_GO_CMD", "gopls"))
		args := splitArgs(getEnv("ONEAGENT_LSP_GO_ARGS", ""))
		return lspServer{Command: cmd, Args: args, Name: "gopls"}
	case lspLangTS:
		cmd := strings.TrimSpace(getEnv("ONEAGENT_LSP_TS_CMD", "typescript-language-server"))
		args := splitArgs(getEnv("ONEAGENT_LSP_TS_ARGS", "--stdio"))
		return lspServer{Command: cmd, Args: args, Name: "typescript-language-server"}
	default:
		return lspServer{}
	}
}

func splitArgs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Keep it simple; callers can avoid spaces by using env vars for tests.
	return strings.Fields(s)
}

func getEnv(k, def string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	return v
}

type lspLocation struct {
	FilePath      string `json:"file_path"`
	Line          int    `json:"line"`           // 1-based
	Character     int    `json:"character"`      // 1-based
	EndLine       int    `json:"end_line"`       // 1-based
	EndCharacter  int    `json:"end_character"`  // 1-based
	PrecedenceURI string `json:"-"`
}

func (a lspLocation) less(b lspLocation) bool {
	if a.FilePath != b.FilePath {
		return a.FilePath < b.FilePath
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Character < b.Character
}

func sortLocations(locs []lspLocation) {
	sort.Slice(locs, func(i, j int) bool { return locs[i].less(locs[j]) })
}

func withinWorkspace(root, abs string) bool {
	root = filepath.Clean(strings.TrimSpace(root))
	abs = filepath.Clean(strings.TrimSpace(abs))
	if root == "" || abs == "" {
		return false
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func relToWorkspace(root, abs string) string {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return abs
	}
	if rel == "." {
		return filepath.Base(abs)
	}
	return filepath.ToSlash(rel)
}

func resolveLSPInputPath(workspaceRoot, input string) (string, error) {
	if strings.TrimSpace(workspaceRoot) == "" {
		return "", scope.ErrWorkspaceNotSet
	}
	return scope.ResolveWritePath(workspaceRoot, input, nil)
}

func ensureServerAvailable(s lspServer) error {
	if strings.TrimSpace(s.Command) == "" {
		return errors.New("lsp server command is empty")
	}
	if filepath.IsAbs(s.Command) {
		return nil
	}
	if _, err := exec.LookPath(s.Command); err != nil {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			name = s.Command
		}
		return fmt.Errorf("language server not found: %s (try `oneagent doctor` and install %s)", s.Command, name)
	}
	return nil
}

func withToolTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	// Keep it bounded; LSP servers can hang under misconfiguration.
	return context.WithTimeout(ctx, 20*time.Second)
}

type locationLink struct {
	TargetURI   string    `json:"targetUri"`
	TargetRange lsp.Range `json:"targetRange"`
}

func parseLocations(raw json.RawMessage) ([]lsp.Location, error) {
	raw = json.RawMessage(bytes.TrimSpace(raw))
	if len(raw) == 0 {
		return nil, errors.New("empty result")
	}
	// []Location
	if raw[0] == '[' {
		var locs []lsp.Location
		if err := json.Unmarshal(raw, &locs); err == nil {
			return locs, nil
		}
		// []LocationLink
		var links []locationLink
		if err := json.Unmarshal(raw, &links); err == nil && len(links) > 0 {
			out := make([]lsp.Location, 0, len(links))
			for _, l := range links {
				out = append(out, lsp.Location{URI: l.TargetURI, Range: l.TargetRange})
			}
			return out, nil
		}
		return nil, errors.New("unsupported location array result")
	}
	// Location
	var one lsp.Location
	if err := json.Unmarshal(raw, &one); err == nil && strings.TrimSpace(one.URI) != "" {
		return []lsp.Location{one}, nil
	}
	// LocationLink
	var link locationLink
	if err := json.Unmarshal(raw, &link); err == nil && strings.TrimSpace(link.TargetURI) != "" {
		return []lsp.Location{{URI: link.TargetURI, Range: link.TargetRange}}, nil
	}
	return nil, errors.New("unsupported location result")
}
