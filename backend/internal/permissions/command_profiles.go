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
			// Baseline (common utils).
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
			"sleep": {},

			// File operations (sandboxed/root-guarded).
			"cp":    {},
			"mv":    {},
			"mkdir": {},
			"rm":    {},
			"touch": {},

			// Developer essentials.
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
			"npx":  {},
			"pnpm": {},
			"yarn": {},

			// Build tools (best-effort).
			"make":    {},
			"cmake":   {},
			"cargo":   {},
			"rustc":   {},
			"rustfmt": {},
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
			"npx":  {},
			"pnpm": {},
			"yarn": {},

			// Build tools (best-effort).
			"make":    {},
			"cmake":   {},
			"cargo":   {},
			"rustc":   {},
			"rustfmt": {},
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
	out := make([]string, 0, 4)
	for _, segment := range splitCommandSegments(command) {
		if token, ok := firstCommandToken(segment); ok {
			out = append(out, token)
		}
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

func splitCommandSegments(command string) []string {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}

	var segments []string
	var cur strings.Builder

	inSingle := false
	inDouble := false
	escaped := false

	flush := func() {
		s := strings.TrimSpace(cur.String())
		cur.Reset()
		if s != "" {
			segments = append(segments, s)
		}
	}

	for i := 0; i < len(command); i++ {
		ch := command[i]

		if escaped {
			escaped = false
			cur.WriteByte(ch)
			continue
		}

		if ch == '\\' && !inSingle {
			escaped = true
			cur.WriteByte(ch)
			continue
		}

		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			cur.WriteByte(ch)
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			cur.WriteByte(ch)
			continue
		}

		if inSingle || inDouble {
			cur.WriteByte(ch)
			continue
		}

		// Outside quotes: recognize command separators.
		if ch == '&' && i+1 < len(command) && command[i+1] == '&' {
			flush()
			i++
			continue
		}
		if ch == '|' && i+1 < len(command) && command[i+1] == '|' {
			flush()
			i++
			continue
		}
		if ch == '&' {
			// Avoid treating common redirection forms as separators (e.g., `2>&1`, `&>out.txt`).
			prev := byte(0)
			if i > 0 {
				prev = command[i-1]
			}
			next := byte(0)
			if i+1 < len(command) {
				next = command[i+1]
			}
			if prev == '>' || prev == '<' || next == '>' || next == '<' {
				cur.WriteByte(ch)
				continue
			}
			flush()
			continue
		}
		if ch == ';' || ch == '|' || ch == '\n' || ch == '\r' || ch == '(' || ch == ')' {
			flush()
			continue
		}

		cur.WriteByte(ch)
	}

	flush()
	return segments
}

func firstCommandToken(segment string) (string, bool) {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return "", false
	}

	inSingle := false
	inDouble := false
	escaped := false
	expectToken := true
	var tok strings.Builder

	flushToken := func() (string, bool) {
		t := strings.TrimSpace(tok.String())
		tok.Reset()
		if t == "" {
			return "", false
		}
		// Skip redirections like `>out`, `<in`, `2>out`, etc.
		if strings.HasPrefix(t, ">") || strings.HasPrefix(t, "<") || strings.HasPrefix(t, "1>") || strings.HasPrefix(t, "2>") {
			return "", false
		}
		// Skip environment assignments like `A=1`.
		if isAssignmentToken(t) {
			return "", false
		}
		return t, true
	}

	for i := 0; i < len(segment); i++ {
		ch := segment[i]

		if escaped {
			escaped = false
			tok.WriteByte(ch)
			continue
		}

		if ch == '\\' && !inSingle {
			escaped = true
			tok.WriteByte(ch)
			continue
		}

		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			tok.WriteByte(ch)
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			tok.WriteByte(ch)
			continue
		}

		if !inSingle && !inDouble && (ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r') {
			if expectToken {
				continue
			}
			if t, ok := flushToken(); ok {
				return t, true
			}
			expectToken = true
			continue
		}

		expectToken = false
		tok.WriteByte(ch)
	}

	if t, ok := flushToken(); ok {
		return t, true
	}
	return "", false
}
