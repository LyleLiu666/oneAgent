package shell

import (
	"fmt"
	"strings"
)

type SandboxMode string

const (
	SandboxModeNone   SandboxMode = "none"
	SandboxModeHost   SandboxMode = "host"
	SandboxModeDocker SandboxMode = "docker"
	SandboxModeNative SandboxMode = "native"
)

func (m SandboxMode) IsHardBoundary() bool {
	return m == SandboxModeDocker || m == SandboxModeNative
}

func ParseSandboxMode(input string) (SandboxMode, error) {
	mode := strings.ToLower(strings.TrimSpace(input))
	if mode == "" {
		mode = string(SandboxModeNone)
	}
	switch mode {
	case string(SandboxModeNone):
		return SandboxModeNone, nil
	case string(SandboxModeHost):
		return SandboxModeHost, nil
	case string(SandboxModeDocker):
		return SandboxModeDocker, nil
	case string(SandboxModeNative):
		return SandboxModeNative, nil
	default:
		return "", fmt.Errorf("unsupported sandbox_mode (expected none|host|docker|native)")
	}
}
