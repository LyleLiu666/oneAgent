package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
)

const (
	defaultTimeout = 10 * time.Second
	maxTimeout     = 30 * time.Second
	maxOutputBytes = 64 * 1024
	maxScriptBytes = 64 * 1024
	defaultRootDir = "./bash-root"
	tmpDirName     = ".tmp"
)

var blacklistedShells = map[string]bool{
	"fish": true,
	"nu":   true,
}

var blockedCommands = map[string]struct{}{
	// Privilege escalation / identity.
	"sudo":      {},
	"su":        {},
	"doas":      {},
	"pkexec":    {},
	"login":     {},
	"logout":    {},
	"passwd":    {},
	"chsh":      {},
	"chfn":      {},
	"newgrp":    {},
	"sg":        {},
	"visudo":    {},
	"useradd":   {},
	"userdel":   {},
	"usermod":   {},
	"groupadd":  {},
	"groupdel":  {},
	"groupmod":  {},
	"command":   {},
	"builtin":   {},
	"type":      {},
	"hash":      {},
	"time":      {},
	"nohup":     {},
	"setsid":    {},
	"stdbuf":    {},
	"timeout":   {},
	"chronic":   {},
	"daemon":    {},
	"daemonize": {},
	"alias":     {},
	"unalias":   {},
	"function":  {},
	"declare":   {},
	"typeset":   {},
	"export":    {},
	"set":       {},
	"unset":     {},
	"readonly":  {},
	"trap":      {},

	// System control / process management.
	"systemctl":  {},
	"service":    {},
	"launchctl":  {},
	"shutdown":   {},
	"reboot":     {},
	"halt":       {},
	"poweroff":   {},
	"init":       {},
	"telinit":    {},
	"kill":       {},
	"killall":    {},
	"pkill":      {},
	"nice":       {},
	"renice":     {},
	"chrt":       {},
	"taskset":    {},
	"chroot":     {},
	"nsenter":    {},
	"unshare":    {},
	"sysctl":     {},
	"modprobe":   {},
	"insmod":     {},
	"rmmod":      {},
	"docker":     {},
	"podman":     {},
	"nerdctl":    {},
	"ctr":        {},
	"containerd": {},
	"runc":       {},
	"lxc":        {},
	"lxd":        {},
	"kubectl":    {},
	"helm":       {},

	// Filesystem / disk manipulation.
	"mount":      {},
	"umount":     {},
	"losetup":    {},
	"fdisk":      {},
	"sfdisk":     {},
	"cfdisk":     {},
	"gdisk":      {},
	"sgdisk":     {},
	"parted":     {},
	"partprobe":  {},
	"growpart":   {},
	"resize2fs":  {},
	"mkfs":       {},
	"mke2fs":     {},
	"mkfs.ext2":  {},
	"mkfs.ext3":  {},
	"mkfs.ext4":  {},
	"mkfs.xfs":   {},
	"mkfs.btrfs": {},
	"mkfs.fat":   {},
	"mkfs.vfat":  {},
	"mkfs.exfat": {},
	"mkfs.ntfs":  {},
	"fsck":       {},
	"e2fsck":     {},
	"fsck.ext2":  {},
	"fsck.ext3":  {},
	"fsck.ext4":  {},
	"fsck.xfs":   {},
	"tune2fs":    {},
	"xfs_repair": {},
	"btrfs":      {},
	"zfs":        {},
	"zpool":      {},
	"lvm":        {},
	"pvcreate":   {},
	"pvremove":   {},
	"vgcreate":   {},
	"vgremove":   {},
	"lvcreate":   {},
	"lvremove":   {},
	"swapon":     {},
	"swapoff":    {},
	"mkswap":     {},
	"diskutil":   {},
	"dd":         {},
	"shred":      {},
	"srm":        {},
	"wipe":       {},

	// Network and remote access.
	"wget2":        {},
	"aria2c":       {},
	"axel":         {},
	"ftp":          {},
	"sftp":         {},
	"scp":          {},
	"ssh":          {},
	"sshpass":      {},
	"rsync":        {},
	"nc":           {},
	"netcat":       {},
	"ncat":         {},
	"socat":        {},
	"telnet":       {},
	"tftp":         {},
	"dig":          {},
	"nslookup":     {},
	"host":         {},
	"whois":        {},
	"ping":         {},
	"traceroute":   {},
	"mtr":          {},
	"ifconfig":     {},
	"ip":           {},
	"ss":           {},
	"netstat":      {},
	"route":        {},
	"arp":          {},
	"tcpdump":      {},
	"tshark":       {},
	"wireshark":    {},
	"nmap":         {},
	"masscan":      {},
	"zmap":         {},
	"iptables":     {},
	"ip6tables":    {},
	"nft":          {},
	"ufw":          {},
	"pfctl":        {},
	"firewall-cmd": {},
	"openssl":      {},

	// Package managers / installers / build tools.
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
	"pip":      {},
	"pip3":     {},
	"conda":    {},
	"mamba":    {},
	// "npm":      {},
	// "pnpm":     {},
	// "yarn":     {},
	// "bun":      {},
	"gem":      {},
	"cargo":    {},
	"rustup":   {},
	"go":       {},
	"composer": {},
	"mvn":      {},
	"gradle":   {},
	"ant":      {},
	"make":     {},
	"cmake":    {},
	"ninja":    {},
	"gcc":      {},
	"g++":      {},
	"clang":    {},
	"clang++":  {},
	"ld":       {},
	"ldconfig": {},

	// VCS.
	"git": {},
	"svn": {},
	"hg":  {},
	"cvs": {},

	// Shells / interpreters / eval.
	"bash":       {},
	"sh":         {},
	"zsh":        {},
	"ksh":        {},
	"dash":       {},
	"ash":        {},
	"csh":        {},
	"tcsh":       {},
	"fish":       {},
	"nu":         {},
	"pwsh":       {},
	"powershell": {},
	"cmd":        {},
	"env":        {},
	"exec":       {},
	"eval":       {},
	"source":     {},
	".":          {},
	"perl":       {},
	"ruby":       {},
	"php":        {},
	"deno":       {},
	"lua":        {},
	"luajit":     {},
	"r":          {},
	"julia":      {},
	"awk":        {},
	"gawk":       {},
	"mawk":       {},
	"nawk":       {},
	"sed":        {},
	"ed":         {},
	"ex":         {},
	"xargs":      {},
	"parallel":   {},

	// Editors / interactive shells.
	"vi":      {},
	"vim":     {},
	"nvim":    {},
	"nano":    {},
	"emacs":   {},
	"tmux":    {},
	"screen":  {},
	"script":  {},
	"expect":  {},
	"busybox": {},
	"toybox":  {},

	// File mutation utilities.
	"rmdir":    {},
	"unlink":   {},
	"mv":       {},
	"cp":       {},
	"ln":       {},
	"install":  {},
	"truncate": {},
	"chmod":    {},
	"chown":    {},
	"chgrp":    {},
	"setfacl":  {},
	"setfattr": {},
	"chattr":   {},
	"tee":      {},
}

// Result captures bash execution details.
type Result struct {
	Shell           string
	Stdout          string
	Stderr          string
	ExitCode        int
	Duration        time.Duration
	TimedOut        bool
	StdoutTruncated bool
	StderrTruncated bool
	CWD             string
}

type tokenKind int

const (
	tokenWord tokenKind = iota
	tokenOperator
	tokenRedirection
)

// UnsafeCommandError indicates the command violates the sandbox policy.
type UnsafeCommandError struct {
	Reason string
}

func (e *UnsafeCommandError) Error() string {
	return e.Reason
}

// ResolveBashRoot returns the sandbox root directory and ensures it exists.
func ResolveBashRoot(rootDir string) (string, error) {
	root := strings.TrimSpace(rootDir)
	if root == "" {
		root = defaultRootDir
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("failed to resolve bash root: %w", err)
	}

	if err := os.MkdirAll(absRoot, 0o700); err != nil {
		return "", fmt.Errorf("failed to create bash root: %w", err)
	}

	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve bash root symlinks: %w", err)
	}

	info, err := os.Stat(realRoot)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("bash root is not a directory: %s", realRoot)
	}

	return realRoot, nil
}

// GuardCommand validates that a bash command stays within the sandbox root.
func GuardCommand(command string, root string) error {
	return guardCommandWithSeen(command, root, make(map[string]struct{}))
}

func guardCommandWithSeen(command string, root string, seen map[string]struct{}) error {
	tokens, err := splitCommandTokens(command)
	if err != nil {
		return &UnsafeCommandError{Reason: err.Error()}
	}

	expectCommand := true
	pendingRedirect := false
	pendingCdTarget := false
	inRm := false
	rmOptionsTerminated := false
	for _, token := range tokens {
		if token.unsafeExpansion {
			return &UnsafeCommandError{
				Reason: "variable and command substitution are not allowed in bash sandbox",
			}
		}

		if pendingRedirect && token.kind != tokenWord {
			return &UnsafeCommandError{Reason: "redirect target missing"}
		}

		switch token.kind {
		case tokenOperator:
			expectCommand = true
			pendingCdTarget = false
			inRm = false
			rmOptionsTerminated = false
			continue
		case tokenRedirection:
			if pendingRedirect {
				return &UnsafeCommandError{Reason: "redirect target missing"}
			}
			if strings.Contains(token.value, "<<") {
				return &UnsafeCommandError{Reason: "heredoc redirection is not allowed"}
			}
			pendingRedirect = true
			continue
		case tokenWord:
			// handled below
		}

		if pendingRedirect {
			if err := validatePathWithinRoot(root, token.value); err != nil {
				return err
			}
			pendingRedirect = false
			continue
		}

		if pendingCdTarget {
			if err := validateCdTargetWithinRoot(root, token.value); err != nil {
				return err
			}
			pendingCdTarget = false
			// cd target is already validated; don't double-check as a generic path token.
			continue
		}

		if inRm {
			if err := validateRmFlags(token.value, &rmOptionsTerminated); err != nil {
				return err
			}
		}

		if expectCommand {
			if !isAssignmentToken(token.value) {
				if err := checkCommandAllowed(token.value, root, seen); err != nil {
					return err
				}
				lower := strings.ToLower(filepath.Base(strings.TrimSpace(token.value)))
				pendingCdTarget = lower == "cd"
				inRm = lower == "rm"
				rmOptionsTerminated = false
				expectCommand = false
			}
		}

		if err := validatePathWithinRoot(root, token.value); err != nil {
			return err
		}
	}

	if pendingRedirect {
		return &UnsafeCommandError{Reason: "redirect target missing"}
	}

	return nil
}

func validateCdTargetWithinRoot(root, raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		// `cd` with no args uses $HOME (which we set to root); safe.
		return nil
	}
	if strings.HasPrefix(value, "-") {
		return &UnsafeCommandError{Reason: "cd flags are not allowed in bash sandbox"}
	}
	if looksLikePath(value) || strings.Contains(value, "..") {
		return validatePathWithinRoot(root, value)
	}
	return validatePathWithinRoot(root, "./"+value)
}

func validateRmFlags(raw string, optionsTerminated *bool) error {
	if optionsTerminated != nil && *optionsTerminated {
		return nil
	}
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	if value == "--" {
		if optionsTerminated != nil {
			*optionsTerminated = true
		}
		return nil
	}
	if strings.HasPrefix(value, "--") {
		if value == "--dereference" || strings.HasPrefix(value, "--dereference=") {
			return &UnsafeCommandError{Reason: "rm dereference flags are not allowed in bash sandbox"}
		}
		return nil
	}
	if strings.HasPrefix(value, "-") && len(value) > 1 {
		if strings.ContainsRune(value[1:], 'L') {
			return &UnsafeCommandError{Reason: "rm dereference flags are not allowed in bash sandbox"}
		}
	}
	return nil
}

func checkCommandAllowed(raw string, root string, seen map[string]struct{}) error {
	name := strings.TrimSpace(raw)
	if name == "" {
		return &UnsafeCommandError{Reason: "empty command"}
	}

	if strings.ContainsAny(name, "(){}") {
		return &UnsafeCommandError{Reason: "function definitions are not allowed"}
	}

	if looksLikePathCommand(name) {
		return inspectExecutableCommand(name, root, seen)
	}

	lower := strings.ToLower(filepath.Base(name))
	if _, blocked := blockedCommands[lower]; blocked {
		return &UnsafeCommandError{Reason: fmt.Sprintf("command %q is not allowed", lower)}
	}

	return nil
}

func looksLikePathCommand(name string) bool {
	if name == "" {
		return false
	}
	return strings.Contains(name, "/") || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "~")
}

func inspectExecutableCommand(name string, root string, seen map[string]struct{}) error {
	normalized, err := normalizePathToken(name, root)
	if err != nil {
		return err
	}
	if normalized == "" {
		return &UnsafeCommandError{Reason: "invalid executable path"}
	}

	absPath := normalized
	if !filepath.IsAbs(normalized) {
		absPath = filepath.Join(root, normalized)
	}
	absPath = filepath.Clean(absPath)

	if !isWithinRoot(root, absPath) {
		return &UnsafeCommandError{
			Reason: fmt.Sprintf("executable %q is outside bash root", name),
		}
	}

	if err := ensurePathWithinRoot(root, absPath); err != nil {
		return err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return &UnsafeCommandError{Reason: fmt.Sprintf("executable %q not found", name)}
	}
	if info.IsDir() {
		return &UnsafeCommandError{Reason: "executable path is a directory"}
	}
	if info.Mode()&0o111 == 0 {
		return &UnsafeCommandError{Reason: "executable is not marked as executable"}
	}

	return inspectExecutableFile(absPath, info, root, seen)
}

func inspectExecutableFile(path string, info os.FileInfo, root string, seen map[string]struct{}) error {
	if _, ok := seen[path]; ok {
		return &UnsafeCommandError{Reason: "recursive script execution is not allowed"}
	}

	if info.Size() > maxScriptBytes {
		return &UnsafeCommandError{Reason: "script is too large to inspect"}
	}

	data, err := readFileLimited(path, maxScriptBytes)
	if err != nil {
		return err
	}

	if isBinaryContent(data) {
		return &UnsafeCommandError{Reason: "binary execution is not allowed"}
	}

	content := string(data)
	if !hasAllowedShebang(content) {
		return &UnsafeCommandError{Reason: "script must start with a bash/sh shebang"}
	}

	sanitized := sanitizeScriptContent(content)
	seen[path] = struct{}{}
	err = guardCommandWithSeen(sanitized, root, seen)
	delete(seen, path)
	return err
}

func readFileLimited(path string, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, &UnsafeCommandError{Reason: "script size limit is invalid"}
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := io.LimitReader(file, limit+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, &UnsafeCommandError{Reason: "script is too large to inspect"}
	}
	return data, nil
}

func isBinaryContent(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

func hasAllowedShebang(content string) bool {
	if content == "" {
		return false
	}
	lineEnd := strings.IndexByte(content, '\n')
	line := content
	if lineEnd >= 0 {
		line = content[:lineEnd]
	}
	if !strings.HasPrefix(line, "#!") {
		return false
	}
	line = strings.TrimSpace(strings.TrimPrefix(line, "#!"))
	if line == "" {
		return false
	}
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return false
	}
	interp := filepath.Base(parts[0])
	if interp == "env" {
		interp = ""
		for i := 1; i < len(parts); i++ {
			if strings.HasPrefix(parts[i], "-") {
				continue
			}
			interp = filepath.Base(parts[i])
			break
		}
	}
	return interp == "bash" || interp == "sh"
}

func sanitizeScriptContent(content string) string {
	lines := strings.Split(content, "\n")
	start := 0
	if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		start = 1
	}

	for i := start; i < len(lines); i++ {
		lines[i] = stripCommentLine(lines[i])
	}

	return strings.Join(lines[start:], "\n")
}

func stripCommentLine(line string) string {
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(line); i++ {
		ch := line[i]

		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}

		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}

		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}

		if ch == '#' && !inSingle && !inDouble {
			return line[:i]
		}
	}

	return line
}

func isAssignmentToken(token string) bool {
	if token == "" || strings.HasPrefix(token, "-") {
		return false
	}

	idx := strings.Index(token, "=")
	if idx <= 0 {
		return false
	}

	name := token[:idx]
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '_' {
			continue
		}
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' {
			continue
		}
		return false
	}

	return true
}

type token struct {
	kind            tokenKind
	value           string
	unsafeExpansion bool
}

func splitCommandTokens(input string) ([]token, error) {
	tokens := make([]token, 0, 8)
	var buf strings.Builder
	inSingle := false
	inDouble := false
	unsafeExpansion := false

	flush := func() {
		if buf.Len() == 0 {
			unsafeExpansion = false
			return
		}
		tokens = append(tokens, token{
			kind:            tokenWord,
			value:           buf.String(),
			unsafeExpansion: unsafeExpansion,
		})
		buf.Reset()
		unsafeExpansion = false
	}

	for i := 0; i < len(input); i++ {
		ch := input[i]

		if inSingle {
			if ch == '\'' {
				inSingle = false
				continue
			}
			buf.WriteByte(ch)
			continue
		}

		if inDouble {
			if ch == '"' {
				inDouble = false
				continue
			}
			if ch == '\\' && i+1 < len(input) {
				i++
				buf.WriteByte(input[i])
				continue
			}
			if ch == '$' || ch == '`' {
				unsafeExpansion = true
			}
			buf.WriteByte(ch)
			continue
		}

		switch ch {
		case '\\':
			if i+1 < len(input) {
				i++
				buf.WriteByte(input[i])
			} else {
				buf.WriteByte(ch)
			}
			continue
		case '\'':
			inSingle = true
			continue
		case '"':
			inDouble = true
			continue
		case ' ', '\t', '\r':
			flush()
			continue
		case '\n':
			flush()
			tokens = append(tokens, token{kind: tokenOperator, value: ";"})
			continue
		}

		if buf.Len() == 0 && isDigit(ch) {
			j := i
			for j < len(input) && isDigit(input[j]) {
				j++
			}
			if j < len(input) && (input[j] == '<' || input[j] == '>') {
				op, consumed := readRedirectionOperator(input, j)
				if consumed > 0 {
					fd := input[i:j]
					tokens = append(tokens, token{kind: tokenRedirection, value: fd + op})
					i = j + consumed - 1
					continue
				}
			}
		}

		if ch == '&' && i+1 < len(input) && input[i+1] == '>' {
			flush()
			op := "&>"
			if i+2 < len(input) && input[i+2] == '>' {
				op = "&>>"
				i += 2
			} else {
				i++
			}
			tokens = append(tokens, token{kind: tokenRedirection, value: op})
			continue
		}

		if isOperatorRune(ch) {
			flush()
			switch ch {
			case ';':
				tokens = append(tokens, token{kind: tokenOperator, value: ";"})
			case '|':
				if i+1 < len(input) && input[i+1] == '|' {
					tokens = append(tokens, token{kind: tokenOperator, value: "||"})
					i++
				} else if i+1 < len(input) && input[i+1] == '&' {
					tokens = append(tokens, token{kind: tokenOperator, value: "|&"})
					i++
				} else {
					tokens = append(tokens, token{kind: tokenOperator, value: "|"})
				}
			case '&':
				if i+1 < len(input) && input[i+1] == '&' {
					tokens = append(tokens, token{kind: tokenOperator, value: "&&"})
					i++
				} else {
					tokens = append(tokens, token{kind: tokenOperator, value: "&"})
				}
			case '<', '>':
				op, consumed := readRedirectionOperator(input, i)
				tokens = append(tokens, token{kind: tokenRedirection, value: op})
				i += consumed - 1
			case '(', ')', '{', '}', '!':
				tokens = append(tokens, token{kind: tokenOperator, value: string(ch)})
			}
			continue
		}

		if ch == '$' || ch == '`' {
			unsafeExpansion = true
		}
		buf.WriteByte(ch)
	}

	if inSingle || inDouble {
		return nil, errors.New("unterminated quote in command")
	}

	flush()
	return tokens, nil
}

func isOperatorToken(token string) bool {
	switch token {
	case "|", "||", "|&", "&&", ";", "&", "(", ")", "{", "}", "!":
		return true
	default:
		return false
	}
}

func isRedirectionOperator(token string) bool {
	if token == "" {
		return false
	}

	ops := map[string]bool{
		">":   true,
		">>":  true,
		"<":   true,
		"<<":  true,
		"<<-": true,
		"<<<": true,
		"<>":  true,
		">|":  true,
		"&>":  true,
		"&>>": true,
		"<&":  true,
		">&":  true,
	}

	if ops[token] {
		return true
	}

	i := 0
	for i < len(token) && token[i] >= '0' && token[i] <= '9' {
		i++
	}
	if i == 0 {
		return false
	}
	return ops[token[i:]]
}

func validatePathWithinRoot(root, token string) error {
	if strings.TrimSpace(token) == "/dev/null" {
		return nil
	}

	normalized, err := normalizePathToken(token, root)
	if err != nil {
		return err
	}
	if normalized == "" {
		return nil
	}

	var absPath string
	if filepath.IsAbs(normalized) {
		absPath = filepath.Clean(normalized)
	} else {
		absPath = filepath.Clean(filepath.Join(root, normalized))
	}

	if !isWithinRoot(root, absPath) {
		return &UnsafeCommandError{
			Reason: fmt.Sprintf("path %q is outside bash root", token),
		}
	}

	if err := ensurePathWithinRoot(root, absPath); err != nil {
		return err
	}

	return nil
}

func isOperatorRune(ch byte) bool {
	switch ch {
	case ';', '|', '&', '<', '>', '(', ')', '{', '}', '!':
		return true
	default:
		return false
	}
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func readRedirectionOperator(input string, index int) (string, int) {
	if index >= len(input) {
		return "", 0
	}
	switch input[index] {
	case '<':
		if index+1 < len(input) {
			switch input[index+1] {
			case '<':
				if index+2 < len(input) {
					switch input[index+2] {
					case '-':
						return "<<-", 3
					case '<':
						return "<<<", 3
					}
				}
				return "<<", 2
			case '>':
				return "<>", 2
			case '&':
				return "<&", 2
			}
		}
		return "<", 1
	case '>':
		if index+1 < len(input) {
			switch input[index+1] {
			case '>':
				return ">>", 2
			case '|':
				return ">|", 2
			case '&':
				return ">&", 2
			}
		}
		return ">", 1
	default:
		return string(input[index]), 1
	}
}

func normalizePathToken(token, root string) (string, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return "", nil
	}

	if idx := strings.Index(trimmed, "="); idx > 0 && strings.HasPrefix(trimmed, "-") {
		suffix := trimmed[idx+1:]
		if looksLikePath(suffix) || strings.Contains(suffix, "..") {
			return normalizePathToken(suffix, root)
		}
		// Not a path-like flag value (e.g. grep patterns). Skip path normalization.
		return "", nil
	}

	isCandidate := looksLikePath(trimmed) || strings.Contains(trimmed, "..")
	if !isCandidate {
		// Skip non-path tokens; GuardCommand validates expansions separately.
		return "", nil
	}

	if (strings.Contains(trimmed, "/") || strings.Contains(trimmed, "..")) && !isPlainPathToken(trimmed) {
		return "", &UnsafeCommandError{Reason: "path token contains unsupported characters"}
	}

	if strings.HasPrefix(trimmed, "~") {
		if trimmed == "~" {
			trimmed = root
		} else if len(trimmed) > 1 && trimmed[1] == '/' {
			trimmed = filepath.Join(root, trimmed[2:])
		} else {
			return "", &UnsafeCommandError{Reason: "tilde user expansion is not allowed"}
		}
	}

	replaced := replaceAllowedVars(trimmed, root)
	if strings.Contains(replaced, "$") || strings.Contains(replaced, "`") {
		return "", &UnsafeCommandError{Reason: "variable expansion is not allowed in paths"}
	}

	return replaced, nil
}

func looksLikePath(value string) bool {
	if value == "" {
		return false
	}

	// Absolute/home/relative paths.
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, ".") || strings.HasPrefix(value, "~") {
		return true
	}

	// Relative paths like "dir/file". Avoid treating obvious non-path tokens (e.g. "</html>") as paths.
	if !strings.Contains(value, "/") {
		return false
	}
	first := value[0]
	if first >= 'a' && first <= 'z' || first >= 'A' && first <= 'Z' || first >= '0' && first <= '9' || first == '_' {
		return true
	}
	return false
}

func isPlainPathToken(value string) bool {
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) {
			continue
		}
		switch r {
		case '/', '.', '_', '-', '~', '*', '?', '[', ']', '{', '}', '+', '@', '%', ':', '=':
			continue
		default:
			return false
		}
	}
	return true
}

func replaceAllowedVars(input, root string) string {
	replacer := strings.NewReplacer(
		"${HOME}", root,
		"$HOME", root,
		"${PWD}", root,
		"$PWD", root,
		"${BASH_ROOT_DIR}", root,
		"$BASH_ROOT_DIR", root,
	)
	return replacer.Replace(input)
}

func isWithinRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if root == target {
		return true
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func ensurePathWithinRoot(root, absPath string) error {
	pathToCheck := absPath
	for {
		if _, err := os.Lstat(pathToCheck); err == nil {
			resolved, err := filepath.EvalSymlinks(pathToCheck)
			if err != nil {
				return &UnsafeCommandError{Reason: "failed to resolve path symlinks"}
			}
			if !isWithinRoot(root, resolved) {
				return &UnsafeCommandError{
					Reason: fmt.Sprintf("path %q resolves outside bash root", absPath),
				}
			}
			return nil
		}
		parent := filepath.Dir(pathToCheck)
		if parent == pathToCheck {
			break
		}
		pathToCheck = parent
	}
	return nil
}

func mergeEnv(base []string, overrides map[string]string) []string {
	env := make([]string, 0, len(base)+len(overrides))

	for _, entry := range base {
		parts := strings.SplitN(entry, "=", 2)
		key := parts[0]
		if _, ok := overrides[key]; ok {
			continue
		}
		env = append(env, entry)
	}

	for key, value := range overrides {
		env = append(env, key+"="+value)
	}

	return env
}

// RunBash executes a bash command and returns output plus metadata.
func RunBash(ctx context.Context, command string, timeout time.Duration, rootDir string, workDir string) (Result, error) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return Result{}, errors.New("command is required")
	}

	root, err := ResolveBashRoot(rootDir)
	if err != nil {
		return Result{}, err
	}

	if err := GuardCommand(trimmed, root); err != nil {
		return Result{}, err
	}

	startDir := root
	if workDir != "" {
		if err := ensurePathWithinRoot(root, workDir); err != nil {
			return Result{}, fmt.Errorf("invalid working directory: %w", err)
		}
		startDir = workDir
	}

	shellPath, err := ResolveBashPath()
	if err != nil {
		return Result{}, err
	}

	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if timeout > maxTimeout {
		timeout = maxTimeout
	}

	tmpDir := filepath.Join(root, tmpDirName)
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("failed to prepare bash temp dir: %w", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	const pwdMarker = "ONEAGENT_PWD_MARKER="
	wrappedCmd := fmt.Sprintf("%s\nRET=$?\necho\necho %s$PWD\nexit $RET", trimmed, pwdMarker)

	cmd := exec.CommandContext(runCtx, shellPath, "--noprofile", "--norc", "-lc", wrappedCmd)
	cmd.Dir = startDir
	cmd.Env = mergeEnv(os.Environ(), map[string]string{
		"HOME":          root,
		"PWD":           root,
		"BASH_ROOT_DIR": root,
		"TMPDIR":        tmpDir,
		"TMP":           tmpDir,
		"TEMP":          tmpDir,
		"BASH_ENV":      "",
	})
	setupCmdForProcessGroup(cmd)

	stdoutBuf := &limitedBuffer{limit: maxOutputBytes}
	stderrBuf := &limitedBuffer{limit: maxOutputBytes}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	timedOut := runCtx.Err() == context.DeadlineExceeded
	if timedOut {
		killProcessTree(cmd.Process)
	}

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		switch {
		case errors.As(runErr, &exitErr):
			exitCode = exitErr.ExitCode()
		case errors.Is(runErr, context.DeadlineExceeded), timedOut:
			exitCode = -1
		default:
			return Result{
				Shell:           shellPath,
				Stdout:          stdoutBuf.String(),
				Stderr:          stderrBuf.String(),
				ExitCode:        exitCode,
				Duration:        duration,
				TimedOut:        timedOut,
				StdoutTruncated: stdoutBuf.Truncated(),
				StderrTruncated: stderrBuf.Truncated(),
			}, runErr
		}
	}

	if timedOut && exitCode == 0 {
		exitCode = -1
	}

	// Parse CWD from stdout
	stdoutStr := stdoutBuf.String()
	cwd := startDir
	if idx := strings.LastIndex(stdoutStr, pwdMarker); idx >= 0 {
		// Extract CWD
		line := stdoutStr[idx+len(pwdMarker):]
		cwd = strings.TrimSpace(line)
		// Remove the marker line and the preceding newline from stdout
		// We added "echo; echo marker", so we should remove the last part
		// Find where the marker started, maybe backtrack to remove the extra newline we added
		cutPoint := idx
		// Check for preceding newline from the first 'echo'
		if cutPoint > 0 && stdoutStr[cutPoint-1] == '\n' {
			cutPoint--
		}
		stdoutStr = stdoutStr[:cutPoint]
	}

	return Result{
		Shell:           shellPath,
		Stdout:          stdoutStr,
		Stderr:          stderrBuf.String(),
		ExitCode:        exitCode,
		Duration:        duration,
		TimedOut:        timedOut,
		StdoutTruncated: stdoutBuf.Truncated(),
		StderrTruncated: stderrBuf.Truncated(),
		CWD:             cwd,
	}, nil
}

func assertShellNotBlacklisted(shellPath string) error {
	base := strings.ToLower(filepath.Base(shellPath))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if blacklistedShells[base] {
		return fmt.Errorf("unsupported shell %q", base)
	}
	return nil
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0111 != 0
}

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if l.limit <= 0 {
		l.truncated = true
		return len(p), nil
	}

	if l.buf.Len()+len(p) <= l.limit {
		return l.buf.Write(p)
	}

	remaining := l.limit - l.buf.Len()
	if remaining > 0 {
		_, _ = l.buf.Write(p[:remaining])
	}
	l.truncated = true
	return len(p), nil
}

func (l *limitedBuffer) String() string {
	return l.buf.String()
}

func (l *limitedBuffer) Truncated() bool {
	return l.truncated
}
