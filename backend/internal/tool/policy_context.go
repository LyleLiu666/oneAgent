package tool

import (
	"context"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

const (
	contextKeyPolicySnapshot contextKey = "policy_snapshot"
)

func ContextWithPolicySnapshot(ctx context.Context, snap permissions.Snapshot) context.Context {
	return context.WithValue(ctx, contextKeyPolicySnapshot, snap)
}

func PolicySnapshotFromContext(ctx context.Context) (permissions.Snapshot, bool) {
	if v, ok := ctx.Value(contextKeyPolicySnapshot).(permissions.Snapshot); ok {
		return v, true
	}
	return permissions.Snapshot{}, false
}

