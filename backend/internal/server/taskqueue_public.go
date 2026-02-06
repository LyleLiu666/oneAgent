package server

import "github.com/liu_y/oneAgent/backend/internal/runtime"

// EnsureTaskQueue configures and starts the TaskQueue runner for a Runtime.
//
// This is useful for non-HTTP entrypoints (e.g. benchmark runners) that still
// want the same attempt execution + evidence artifacts.
func EnsureTaskQueue(rt *runtime.Runtime) error {
	return ensureTaskQueue(rt)
}

