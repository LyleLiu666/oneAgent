package settingsdb

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

func TestToolPolicy_SetGet(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectAllow,
		Rules: []permissions.Rule{
			{ID: "deny-bash", Effect: permissions.EffectDeny, ToolID: "bash"},
		},
	}

	if err := db.SetToolPolicy(ctx, "alice", policy); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, ok, err := db.GetToolPolicy(ctx, "alice")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok || got.ID != "p1" {
		t.Fatalf("unexpected policy: ok=%v policy=%+v", ok, got)
	}
}

