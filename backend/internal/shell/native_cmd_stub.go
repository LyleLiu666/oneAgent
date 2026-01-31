//go:build !darwin

package shell

import (
	"errors"
	"os/exec"
)

func newNativeBashCommand(_, _, _, _ string) (*exec.Cmd, string, error) {
	return nil, "", errors.New("native sandbox is not supported on this platform (use sandbox_mode=docker)")
}

