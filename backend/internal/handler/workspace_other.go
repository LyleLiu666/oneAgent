//go:build !darwin && !windows

package handler

import (
	"context"
	"fmt"
	"runtime"
)

func defaultChooseWorkspaceDir(ctx context.Context) (string, bool, error) {
	_ = ctx
	return "", false, fmt.Errorf("%w: %s", ErrWorkspaceChooserNotSupported, runtime.GOOS)
}
