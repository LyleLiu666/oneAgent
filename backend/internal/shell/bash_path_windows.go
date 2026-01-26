//go:build windows

package shell

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var portableGitInstallMu sync.Mutex

func ResolveBashPath() (string, error) {
	resolved, err := ResolveBashBinary()
	if err != nil {
		return "", err
	}
	return resolved.Path, nil
}

func ResolveBashBinary() (ResolvedBinary, error) {
	if explicit := strings.TrimSpace(os.Getenv("LYLE_BASH_PATH")); explicit != "" {
		if err := assertShellNotBlacklisted(explicit); err != nil {
			return ResolvedBinary{}, err
		}
		if !isExecutable(explicit) {
			return ResolvedBinary{}, fmt.Errorf("bash not found at: %s", explicit)
		}
		return ResolvedBinary{Path: explicit, Source: BinarySourceExplicit}, nil
	}

	if resolved, ok := findBundledGitBash(); ok {
		return resolved, nil
	}

	if resolved, ok := ensureBundledGitBash(); ok {
		return resolved, nil
	}

	if resolved, ok := findGitBashFromSystemGit(); ok {
		return resolved, nil
	}

	return ResolvedBinary{}, errors.New("git bash not found: install Git for Windows or use a oneAgent Windows release that bundles PortableGit")
}

func ResolveBashBinaryNoInstall() (ResolvedBinary, error) {
	if explicit := strings.TrimSpace(os.Getenv("LYLE_BASH_PATH")); explicit != "" {
		if err := assertShellNotBlacklisted(explicit); err != nil {
			return ResolvedBinary{}, err
		}
		if !isExecutable(explicit) {
			return ResolvedBinary{}, fmt.Errorf("bash not found at: %s", explicit)
		}
		return ResolvedBinary{Path: explicit, Source: BinarySourceExplicit}, nil
	}

	if resolved, ok := findBundledGitBash(); ok {
		return resolved, nil
	}

	if resolved, ok := findGitBashFromSystemGit(); ok {
		return resolved, nil
	}

	return ResolvedBinary{}, errors.New("git bash not found")
}

func findBundledGitBash() (ResolvedBinary, bool) {
	if gitRoot := bundledGitRootFromHome(); gitRoot != "" {
		if bash := findGitBashInRoot(gitRoot); bash != "" {
			return ResolvedBinary{Path: bash, Source: BinarySourceBundled}, true
		}
	}

	if gitRoot := bundledGitRootNextToExecutable(); gitRoot != "" {
		if bash := findGitBashInRoot(gitRoot); bash != "" {
			return ResolvedBinary{Path: bash, Source: BinarySourceBundled}, true
		}
	}

	return ResolvedBinary{}, false
}

func bundledGitRootFromHome() string {
	home := resolveOneAgentHome()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".oneagent", "bundled", "git")
}

func bundledGitRootNextToExecutable() string {
	exe, err := os.Executable()
	if err != nil || strings.TrimSpace(exe) == "" {
		return ""
	}
	exeDir := filepath.Dir(exe)
	return filepath.Join(exeDir, "bundled", "git")
}

func findGitBashFromSystemGit() (ResolvedBinary, bool) {
	if gitPath, err := exec.LookPath("git"); err == nil && strings.TrimSpace(gitPath) != "" {
		if bash := findGitBashNearGit(gitPath); bash != "" {
			return ResolvedBinary{Path: bash, Source: BinarySourceSystem}, true
		}
	}

	roots := make([]string, 0, 4)
	if pf := strings.TrimSpace(os.Getenv("ProgramFiles")); pf != "" {
		roots = append(roots, filepath.Join(pf, "Git"))
	}
	if pf := strings.TrimSpace(os.Getenv("ProgramFiles(x86)")); pf != "" {
		roots = append(roots, filepath.Join(pf, "Git"))
	}
	if local := strings.TrimSpace(os.Getenv("LocalAppData")); local != "" {
		roots = append(roots, filepath.Join(local, "Programs", "Git"))
	}

	for _, root := range roots {
		if bash := findGitBashInRoot(root); bash != "" {
			return ResolvedBinary{Path: bash, Source: BinarySourceSystem}, true
		}
	}

	return ResolvedBinary{}, false
}

func findGitBashNearGit(gitExe string) string {
	gitExe = strings.TrimSpace(gitExe)
	if gitExe == "" {
		return ""
	}

	dir := filepath.Dir(gitExe)
	base := strings.ToLower(filepath.Base(dir))

	candidates := []string{dir}
	if base == "cmd" || base == "bin" {
		candidates = append(candidates, filepath.Dir(dir))
	}
	candidates = append(candidates, filepath.Dir(dir))
	candidates = append(candidates, filepath.Dir(filepath.Dir(dir)))

	seen := make(map[string]struct{}, len(candidates))
	for _, root := range candidates {
		root = filepath.Clean(root)
		if root == "" {
			continue
		}
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}
		if bash := findGitBashInRoot(root); bash != "" {
			return bash
		}
	}

	return ""
}

func findGitBashInRoot(gitRoot string) string {
	gitRoot = strings.TrimSpace(gitRoot)
	if gitRoot == "" {
		return ""
	}

	candidates := []string{
		filepath.Join(gitRoot, "usr", "bin", "bash.exe"),
		filepath.Join(gitRoot, "bin", "bash.exe"),
	}

	for _, c := range candidates {
		if isExecutable(c) {
			return c
		}
	}

	return ""
}

func ensureBundledGitBash() (ResolvedBinary, bool) {
	portableGitInstallMu.Lock()
	defer portableGitInstallMu.Unlock()

	gitRoot := bundledGitRootFromHome()
	if gitRoot == "" {
		return ResolvedBinary{}, false
	}

	if bash := findGitBashInRoot(gitRoot); bash != "" {
		return ResolvedBinary{Path: bash, Source: BinarySourceBundled}, true
	}

	archive := PortableGitArchivePath()
	if archive == "" {
		return ResolvedBinary{}, false
	}

	if err := os.MkdirAll(gitRoot, 0o700); err != nil {
		return ResolvedBinary{}, false
	}

	installPath := strings.ReplaceAll(gitRoot, `\`, `\\`)
	argInstall := "-InstallPath=" + installPath

	cmd := exec.Command(archive, "-y", "-gm2", argInstall)
	cmd.Dir = filepath.Dir(archive)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return ResolvedBinary{}, false
	}

	if bash := findGitBashInRoot(gitRoot); bash != "" {
		return ResolvedBinary{Path: bash, Source: BinarySourceBundled}, true
	}

	return ResolvedBinary{}, false
}

func PortableGitArchivePath() string {
	exe, err := os.Executable()
	if err != nil || strings.TrimSpace(exe) == "" {
		return ""
	}
	exeDir := filepath.Dir(exe)

	patterns := []string{
		filepath.Join(exeDir, "bundled", "PortableGit.7z.exe"),
		filepath.Join(exeDir, "bundled", "PortableGit-*-64-bit.7z.exe"),
		filepath.Join(exeDir, "PortableGit.7z.exe"),
		filepath.Join(exeDir, "PortableGit-*-64-bit.7z.exe"),
	}

	for _, pat := range patterns {
		if strings.Contains(pat, "*") {
			matches, _ := filepath.Glob(pat)
			for _, m := range matches {
				if isExecutable(m) {
					return m
				}
			}
			continue
		}
		if isExecutable(pat) {
			return pat
		}
	}

	return ""
}

func resolveOneAgentHome() string {
	if home := strings.TrimSpace(os.Getenv("ONEAGENT_HOME")); home != "" {
		return home
	}
	userHome, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(userHome) == "" {
		return ""
	}
	return filepath.Join(userHome, ".oneagent_default")
}
