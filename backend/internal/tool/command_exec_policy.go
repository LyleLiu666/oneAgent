package tool

import (
	"fmt"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/shell"
)

type commandExecPolicy struct {
	mode      shell.SandboxMode
	profile   string
	allowlist []string
}

func resolveCommandExecPolicy(dec permissions.Decision, command string) (commandExecPolicy, error) {
	mode, err := commandToolSandboxMode(dec)
	if err != nil {
		return commandExecPolicy{}, err
	}

	profile := CommandProfile(dec, "dev")
	allowlist := dec.Constraints.Allowlist

	// System installers are explicitly allowed to run on the host (no sandbox),
	// as long as the policy isn't already restricting commands via allowlist.
	if isSystemInstallCommand(command) && len(allowlist) == 0 {
		profile = "system_install"
		mode = shell.SandboxModeHost
	}

	requestedProfile := strings.ToLower(strings.TrimSpace(profile))
	if requestedProfile == "coding" && !mode.IsHardBoundary() {
		return commandExecPolicy{}, fmt.Errorf("coding profile requires sandbox_mode=docker|native (got %s)", mode)
	}

	if mode == shell.SandboxModeNone {
		// Safety invariant: without a hard sandbox boundary, command tools MUST degrade to readonly.
		profile = "readonly"
		allowlist = nil
	}

	return commandExecPolicy{
		mode:      mode,
		profile:   profile,
		allowlist: allowlist,
	}, nil
}
