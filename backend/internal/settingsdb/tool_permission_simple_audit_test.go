package settingsdb

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

func TestApplySimpleToolPermissionChange_PersistsPolicyApprovalAndAudit(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	oldPolicy := permissions.Policy{
		ID:                "simple_readonly",
		DefaultEffect:     permissions.EffectAllow,
		DefaultCmdProfile: "readonly",
	}
	newPolicy := permissions.Policy{
		ID:                "simple_host_full",
		DefaultEffect:     permissions.EffectAllow,
		DefaultCmdProfile: "full",
	}

	if err := db.ApplySimpleToolPermissionChange(ctx, ToolPermissionSimpleAuditRecord{
		PrincipalID:         "alice",
		Source:              "chat_header",
		OldPolicy:           oldPolicy,
		NewPolicy:           newPolicy,
		SelectedMode:        string(permissions.SimpleModeHostFull),
		CommandApprovalMode: "manual",
		ChangedAt:           time.Unix(1_700_000_000, 0).UTC(),
	}); err != nil {
		t.Fatalf("apply simple tool permission change: %v", err)
	}

	policy, ok, err := db.GetToolPolicy(ctx, "alice")
	if err != nil {
		t.Fatalf("get tool policy: %v", err)
	}
	if !ok || policy.ID != "simple_host_full" {
		t.Fatalf("unexpected persisted policy: ok=%v policy=%+v", ok, policy)
	}

	mode, err := db.GetUserSetting(ctx, "alice", SettingKeyCommandApprovalMode)
	if err != nil {
		t.Fatalf("get command approval mode: %v", err)
	}
	if mode != "manual" {
		t.Fatalf("expected manual approval mode, got %q", mode)
	}

	audit, err := db.ListSimpleToolPermissionAudit(ctx, "alice")
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(audit) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(audit))
	}
	if audit[0].Source != "chat_header" || audit[0].SelectedMode != string(permissions.SimpleModeHostFull) {
		t.Fatalf("unexpected audit row: %+v", audit[0])
	}
	if audit[0].OldPolicy.ID != "simple_readonly" || audit[0].NewPolicy.ID != "simple_host_full" {
		t.Fatalf("unexpected audit policy transition: %+v", audit[0])
	}
}

func TestApplySimpleToolPermissionChange_SkipsPolicyPersistenceWhenRequested(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	oldPolicy := permissions.Policy{
		ID:                "default",
		DefaultEffect:     permissions.EffectAllow,
		DefaultCmdProfile: "dev",
	}

	if err := db.ApplySimpleToolPermissionChange(ctx, ToolPermissionSimpleAuditRecord{
		PrincipalID:           "alice",
		Source:                "chat_header",
		OldPolicy:             oldPolicy,
		NewPolicy:             oldPolicy,
		SelectedMode:          string(permissions.SimpleModeCustom),
		CommandApprovalMode:   "manual",
		SkipPolicyPersistence: true,
		ChangedAt:             time.Unix(1_700_000_100, 0).UTC(),
	}); err != nil {
		t.Fatalf("apply simple tool permission change: %v", err)
	}

	if _, ok, err := db.GetToolPolicy(ctx, "alice"); err != nil {
		t.Fatalf("get tool policy: %v", err)
	} else if ok {
		t.Fatalf("expected tool policy to remain implicit")
	}

	mode, err := db.GetUserSetting(ctx, "alice", SettingKeyCommandApprovalMode)
	if err != nil {
		t.Fatalf("get command approval mode: %v", err)
	}
	if mode != "manual" {
		t.Fatalf("expected manual approval mode, got %q", mode)
	}

	audit, err := db.ListSimpleToolPermissionAudit(ctx, "alice")
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(audit) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(audit))
	}
	if audit[0].OldPolicy.ID != "default" || audit[0].NewPolicy.ID != "default" {
		t.Fatalf("unexpected audit policy snapshot: %+v", audit[0])
	}
}
