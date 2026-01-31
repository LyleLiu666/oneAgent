package tool

import (
	"strings"
)

// isHighRiskCommand reports whether a command should be approval-gated as "high risk".
//
// v1 classification is intentionally simple and token-based: if any command segment uses a
// high-risk binary (delete/move, installers, interpreters/shells), the whole command is treated
// as high-risk.
func isHighRiskCommand(command string) bool {
	for _, seg := range extractCommandSegments(command) {
		if seg.cmd == "" {
			continue
		}
		if _, ok := highRiskCommandTokens[seg.cmd]; ok {
			return true
		}
		if seg.cmd == "git" && isHighRiskGit(seg.args) {
			return true
		}
		if seg.cmd == "find" && isHighRiskFind(seg.args) {
			return true
		}
	}
	return false
}

var highRiskCommandTokens = map[string]struct{}{
	// Destructive file operations.
	"rm":    {},
	"mv":    {},
	"rmdir": {},
	"del":   {}, // windows (best-effort)

	// OS / package installers.
	"apt":     {},
	"apt-get": {},
	"yum":     {},
	"dnf":     {},
	"pacman":  {},
	"brew":    {},

	// Language / project package managers.
	"npm":    {},
	"pnpm":   {},
	"yarn":   {},
	"npx":    {},
	"pip":    {},
	"pip3":   {},
	"pipx":   {},
	"poetry": {},

	// Interpreters / shells (arbitrary code execution).
	"python":     {},
	"python3":    {},
	"node":       {},
	"sh":         {},
	"bash":       {},
	"zsh":        {},
	"pwsh":       {},
	"powershell": {},
}

type commandSegment struct {
	cmd  string
	args []string
}

func extractCommandSegments(command string) []commandSegment {
	// Keep behavior in sync with permissions.ExtractCommands (best-effort), but preserve arguments
	// so we can detect subcommand-style dangerous operations (e.g., `git reset`, `find -delete`).
	repl := strings.NewReplacer(
		"&&", " && ",
		"||", " || ",
		";", " ; ",
		"|", " | ",
		"&", " & ",
		"\n", " \n ",
		"\r", " \r ",
		"(", " ( ",
		")", " ) ",
	)
	parts := strings.Fields(repl.Replace(command))
	out := make([]commandSegment, 0, len(parts))

	expectCommand := true
	current := commandSegment{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if isSeparatorToken(part) {
			if current.cmd != "" {
				out = append(out, current)
			}
			current = commandSegment{}
			expectCommand = true
			continue
		}
		if expectCommand {
			if strings.HasPrefix(part, ">") || strings.HasPrefix(part, "<") {
				continue
			}
			if isAssignmentToken(part) {
				continue
			}
			current.cmd = normalizeCommandToken(part)
			expectCommand = false
			continue
		}
		current.args = append(current.args, part)
	}
	if current.cmd != "" {
		out = append(out, current)
	}
	return out
}

func normalizeCommandToken(raw string) string {
	t := strings.ToLower(strings.TrimSpace(raw))
	if t == "" {
		return ""
	}
	t = strings.TrimPrefix(t, "./")
	t = strings.ReplaceAll(t, "\\", "/")
	if idx := strings.LastIndex(t, "/"); idx >= 0 {
		t = t[idx+1:]
	}
	return t
}

func isSeparatorToken(token string) bool {
	switch token {
	case ";", "&&", "||", "|", "&", "\n", "\r", "(", ")":
		return true
	default:
		return false
	}
}

func isAssignmentToken(token string) bool {
	return strings.Contains(token, "=") && !strings.HasPrefix(token, "./") && !strings.HasPrefix(token, "/")
}

func isHighRiskGit(args []string) bool {
	sub, rest := gitSubcommand(args)
	switch sub {
	case "clean":
		// `git clean` requires -f to actually delete; treat only forced variants as high-risk.
		for _, a := range rest {
			la := strings.ToLower(strings.TrimSpace(a))
			if la == "" {
				continue
			}
			if la == "--force" {
				return true
			}
			if strings.HasPrefix(la, "-") && strings.Contains(la[1:], "f") {
				return true
			}
		}
		return false
	case "reset":
		// `git reset` can discard local work; be conservative.
		return true
	default:
		return false
	}
}

func gitSubcommand(args []string) (string, []string) {
	// Best-effort parse: skip common global options that take an argument.
	skipNext := false
	for i := 0; i < len(args); i++ {
		if skipNext {
			skipNext = false
			continue
		}
		a := strings.TrimSpace(args[i])
		if a == "" {
			continue
		}
		switch a {
		case "-C", "--git-dir", "--work-tree", "-c":
			skipNext = true
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		return strings.ToLower(a), args[i+1:]
	}
	return "", nil
}

func isHighRiskFind(args []string) bool {
	for _, a := range args {
		la := strings.ToLower(strings.TrimSpace(a))
		switch la {
		case "-delete", "-exec", "-ok":
			return true
		}
	}
	return false
}
