package permissions

import (
	"fmt"
	"strings"
)

func AllowedCommands(profile string) map[string]struct{} {
	profile = strings.TrimSpace(strings.ToLower(profile))
	switch profile {
	case "full":
		return nil
	case "readonly":
		return map[string]struct{}{
			"cd":    {},
			"ls":    {},
			"cat":   {},
			"rg":    {},
			"grep":  {},
			"pwd":   {},
			"echo":  {},
			"sleep": {},
			"head":  {},
			"tail":  {},
			"wc":    {},
			"sort":  {},
			"uniq":  {},
		}
	case "system_install":
		return map[string]struct{}{
			// Baseline (mostly read-only utils).
			"cd":    {},
			"ls":    {},
			"cat":   {},
			"rg":    {},
			"grep":  {},
			"pwd":   {},
			"echo":  {},
			"head":  {},
			"tail":  {},
			"wc":    {},
			"sort":  {},
			"uniq":  {},
			"sleep": {},

			// System package managers (high-risk; approval-gated).
			"apt":      {},
			"apt-get":  {},
			"aptitude": {},
			"yum":      {},
			"dnf":      {},
			"pacman":   {},
			"apk":      {},
			"zypper":   {},
			"brew":     {},
			"port":     {},
			"snap":     {},
			"flatpak":  {},
			"pkg":      {},
			"pkgutil":  {},
		}
	case "dev":
		return map[string]struct{}{
			"cd":    {},
			"ls":    {},
			"cat":   {},
			"rg":    {},
			"grep":  {},
			"pwd":   {},
			"echo":  {},
			"head":  {},
			"tail":  {},
			"wc":    {},
			"sort":  {},
			"uniq":  {},
			"sed":   {},
			"awk":   {},
			"cp":    {},
			"mv":    {},
			"mkdir": {},
			"rm":    {},
			"touch": {},
			"sleep": {},
		}
	case "coding":
		return map[string]struct{}{
			// Baseline (dev-like).
			"cd":    {},
			"ls":    {},
			"cat":   {},
			"rg":    {},
			"grep":  {},
			"pwd":   {},
			"echo":  {},
			"head":  {},
			"tail":  {},
			"wc":    {},
			"sort":  {},
			"uniq":  {},
			"sed":   {},
			"awk":   {},
			"cp":    {},
			"mv":    {},
			"mkdir": {},
			"rm":    {},
			"touch": {},
			"sleep": {},

			// Coding essentials.
			"find": {},
			"git":  {},

			// Go.
			"go":    {},
			"gofmt": {},

			// Python.
			"python":  {},
			"python3": {},
			"pip":     {},
			"pip3":    {},
			"pytest":  {},

			// Node.
			"node": {},
			"npm":  {},
			"pnpm": {},
			"yarn": {},

			// Build tools (best-effort).
			"make":  {},
			"cmake": {},
			"cargo": {},
		}
	default:
		return map[string]struct{}{
			"ls":   {},
			"cat":  {},
			"rg":   {},
			"pwd":  {},
			"echo": {},
		}
	}
}

func ValidateCommand(profile string, command string, allowlist []string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("command is required")
	}
	profile = strings.TrimSpace(strings.ToLower(profile))
	if profile == "" {
		profile = "dev"
	}
	if profile == "readonly" && containsRedirectionOutsideQuotes(command) {
		return fmt.Errorf("command not allowed by profile=%s: redirection", profile)
	}

	allowed := AllowedCommands(profile)
	if len(allowlist) > 0 {
		allowed = make(map[string]struct{}, len(allowlist))
		for _, cmd := range allowlist {
			cmd = strings.TrimSpace(cmd)
			if cmd != "" {
				allowed[cmd] = struct{}{}
			}
		}
	}

	// profile=full -> no allowlist enforcement.
	if allowed == nil {
		return nil
	}

	for _, token := range extractCommands(command) {
		if _, ok := allowed[token]; !ok {
			return fmt.Errorf("command not allowed by profile=%s: %s", profile, token)
		}
	}
	return nil
}

// ExtractCommands returns the first token of each command segment, using the same logic as ValidateCommand.
//
// This is useful for higher-level safety checks (e.g., high-risk command classification).
func ExtractCommands(command string) []string {
	return extractCommands(command)
}

// extractCommands returns the first token of each command segment.
func extractCommands(command string) []string {
	command = protectRedirectionAmpersands(command)
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
	cleaned := repl.Replace(command)
	parts := strings.Fields(cleaned)
	out := make([]string, 0, len(parts))
	expectCommand := true
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if isSeparatorToken(part) {
			expectCommand = true
			continue
		}
		if !expectCommand {
			continue
		}
		if strings.HasPrefix(part, ">") || strings.HasPrefix(part, "<") {
			continue
		}
		if isAssignmentToken(part) {
			continue
		}
		out = append(out, part)
		expectCommand = false
	}
	return out
}

func protectRedirectionAmpersands(command string) string {
	// Avoid treating `&` in common redirection forms (e.g., `2>&1`, `&>out.txt`) as a command separator.
	//
	// This is a best-effort lexical tweak for our command token extraction; it does not change execution.
	const sentinel = "§"
	return strings.NewReplacer(
		"&>>", sentinel+">>",
		"&>", sentinel+">",
		">&", ">"+sentinel,
		"<&", "<"+sentinel,
	).Replace(command)
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

func containsRedirectionOutsideQuotes(command string) bool {
	inSingle := false
	inDouble := false
	escaped := false
	for _, r := range command {
		if escaped {
			escaped = false
			continue
		}
		switch r {
		case '\\':
			escaped = true
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '<', '>':
			if !inSingle && !inDouble {
				return true
			}
		}
	}
	return false
}
