package runtime

import (
	"context"
	"errors"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

// ResolveToolPolicySnapshot resolves an effective tool policy snapshot for a principal.
func (r *Runtime) ResolveToolPolicySnapshot(ctx context.Context, principalID string) (permissions.Snapshot, error) {
	if r == nil {
		return permissions.Snapshot{}, errors.New("runtime is nil")
	}
	policy := permissions.DefaultPolicy()
	if r.Settings != nil {
		got, ok, err := r.Settings.GetToolPolicy(ctx, principalID)
		if err != nil {
			return permissions.Snapshot{}, err
		}
		if ok {
			policy = got
		}
	}
	return permissions.ResolveSnapshot(principalID, policy, time.Now()), nil
}
