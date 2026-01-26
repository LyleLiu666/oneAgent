//go:build windows

package shell

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

func ResolveGitBinaryNoInstall() (ResolvedBinary, error) {
	if gitRoot := bundledGitRootFromHome(); gitRoot != "" {
		if git := findGitExeInRoot(gitRoot); git != "" {
			return ResolvedBinary{Path: git, Source: BinarySourceBundled}, nil
		}
	}

	if gitRoot := bundledGitRootNextToExecutable(); gitRoot != "" {
		if git := findGitExeInRoot(gitRoot); git != "" {
			return ResolvedBinary{Path: git, Source: BinarySourceBundled}, nil
		}
	}

	if gitPath, err := exec.LookPath("git"); err == nil && strings.TrimSpace(gitPath) != "" {
		return ResolvedBinary{Path: gitPath, Source: BinarySourceSystem}, nil
	}

	return ResolvedBinary{}, errors.New("git not found")
}

func findGitExeInRoot(gitRoot string) string {
	gitRoot = strings.TrimSpace(gitRoot)
	if gitRoot == "" {
		return ""
	}

	candidates := []string{
		filepath.Join(gitRoot, "cmd", "git.exe"),
		filepath.Join(gitRoot, "bin", "git.exe"),
	}

	for _, c := range candidates {
		if isExecutable(c) {
			return c
		}
	}

	return ""
}
