//go:build !darwin && !linux && !windows

package shell

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

func RunBashNative(_ context.Context, _ string, _ time.Duration, _ string, _ string) (Result, error) {
	return Result{}, fmt.Errorf("native sandbox is not supported on %s (use sandbox_mode=docker)", runtime.GOOS)
}
