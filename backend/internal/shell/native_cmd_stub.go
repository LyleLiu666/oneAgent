//go:build !darwin && !linux && !windows

package shell

import (
	"errors"
	"os/exec"
)

func newNativeBashCommand(_, _, _, _ string) (*exec.Cmd, func(), error) {
	return nil, nil, errors.New("native sandbox is not supported on this platform (use sandbox_mode=docker)")
}
