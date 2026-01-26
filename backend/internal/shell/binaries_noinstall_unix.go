//go:build !windows

package shell

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

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

	path, err := ResolveBashPath()
	if err != nil {
		return ResolvedBinary{}, err
	}
	return ResolvedBinary{Path: path, Source: BinarySourceSystem}, nil
}

func ResolveGitBinaryNoInstall() (ResolvedBinary, error) {
	if path, err := exec.LookPath("git"); err == nil && strings.TrimSpace(path) != "" {
		return ResolvedBinary{Path: path, Source: BinarySourceSystem}, nil
	}
	return ResolvedBinary{}, errors.New("git not found")
}

func PortableGitArchivePath() string {
	return ""
}
