package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
)

func TestToolApprovalGate_WriteFile_RequiredApproveThenConsume(t *testing.T) {
	db, err := settingsdb.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("open settings db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{
			{
				ID:     "allow-write-file-with-approval",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDWriteFile,
				Constraints: permissions.Constraints{
					Approval: "required",
				},
			},
		},
	}
	snap := permissions.ResolveSnapshot("alice", policy, time.Now())

	defs, err := MountWithSnapshot([]string{ToolIDWriteFile}, snap)
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(defs))
	}

	ws := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: ws})
	ctx = ContextWithSessionID(ctx, "session-1")
	ctx = ContextWithSettingsDB(ctx, db)

	raw, _ := json.Marshal(map[string]any{
		"filePath": "a.txt",
		"content":  "hello\n",
	})

	_, err = defs[0].Handler(ctx, raw)
	if err == nil {
		t.Fatalf("expected approval required error")
	}
	var reqErr *ApprovalRequiredError
	if !errors.As(err, &reqErr) {
		t.Fatalf("expected ApprovalRequiredError, got %T (%v)", err, err)
	}
	if reqErr.ApprovalID == "" {
		t.Fatalf("expected approval id")
	}
	if _, statErr := os.Stat(filepath.Join(ws, "a.txt")); statErr == nil {
		t.Fatalf("expected file not to be written before approval")
	}

	if err := db.ApproveToolApproval(context.Background(), reqErr.ApprovalID, "ok"); err != nil {
		t.Fatalf("approve: %v", err)
	}

	_, err = defs[0].Handler(ctx, raw)
	if err != nil {
		t.Fatalf("expected write to succeed after approval, got %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(ws, "a.txt"))
	if readErr != nil {
		t.Fatalf("read written file: %v", readErr)
	}
	if string(data) != "hello\n" {
		t.Fatalf("unexpected file content: %q", string(data))
	}

	// Consumed: same call requires a new approval.
	_, err = defs[0].Handler(ctx, raw)
	if err == nil {
		t.Fatalf("expected approval required after consume")
	}
	if !errors.As(err, &reqErr) {
		t.Fatalf("expected ApprovalRequiredError after consume, got %T (%v)", err, err)
	}

	// Scope: approval must not apply across sessions.
	ctx2 := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: ws})
	ctx2 = ContextWithSessionID(ctx2, "session-2")
	ctx2 = ContextWithSettingsDB(ctx2, db)
	_, err = defs[0].Handler(ctx2, raw)
	if err == nil {
		t.Fatalf("expected approval required for a different session scope")
	}
}

func TestToolApprovalGate_WriteFile_AttemptScopeTakesPrecedenceOverSession(t *testing.T) {
	db, err := settingsdb.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("open settings db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{
			{
				ID:     "allow-write-file-with-approval",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDWriteFile,
				Constraints: permissions.Constraints{
					Approval: "required",
				},
			},
		},
	}
	snap := permissions.ResolveSnapshot("alice", policy, time.Now())

	defs, err := MountWithSnapshot([]string{ToolIDWriteFile}, snap)
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(defs))
	}

	ws := t.TempDir()
	raw, _ := json.Marshal(map[string]any{
		"filePath": "a.txt",
		"content":  "hello\n",
	})

	ctxAttempt1 := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: ws})
	ctxAttempt1 = ContextWithSessionID(ctxAttempt1, "session-1")
	ctxAttempt1 = ContextWithAttemptID(ctxAttempt1, "attempt-1")
	ctxAttempt1 = ContextWithSettingsDB(ctxAttempt1, db)

	_, err = defs[0].Handler(ctxAttempt1, raw)
	if err == nil {
		t.Fatalf("expected approval required error")
	}
	var reqErr *ApprovalRequiredError
	if !errors.As(err, &reqErr) {
		t.Fatalf("expected ApprovalRequiredError, got %T (%v)", err, err)
	}
	if reqErr.ApprovalID == "" {
		t.Fatalf("expected approval id")
	}

	if err := db.ApproveToolApproval(context.Background(), reqErr.ApprovalID, "ok"); err != nil {
		t.Fatalf("approve: %v", err)
	}

	ctxAttempt2 := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: ws})
	ctxAttempt2 = ContextWithSessionID(ctxAttempt2, "session-1")
	ctxAttempt2 = ContextWithAttemptID(ctxAttempt2, "attempt-2")
	ctxAttempt2 = ContextWithSettingsDB(ctxAttempt2, db)

	_, err = defs[0].Handler(ctxAttempt2, raw)
	if err == nil {
		t.Fatalf("expected approval required for a different attempt scope")
	}
	if !errors.As(err, &reqErr) {
		t.Fatalf("expected ApprovalRequiredError, got %T (%v)", err, err)
	}
	if _, statErr := os.Stat(filepath.Join(ws, "a.txt")); statErr == nil {
		t.Fatalf("expected file not to be written before approval")
	}
}

func TestToolApprovalGate_WriteFile_DeniedBlocksSameFingerprint(t *testing.T) {
	db, err := settingsdb.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("open settings db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{
			{
				ID:     "allow-write-file-with-approval",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDWriteFile,
				Constraints: permissions.Constraints{
					Approval: "required",
				},
			},
		},
	}
	snap := permissions.ResolveSnapshot("alice", policy, time.Now())

	defs, err := MountWithSnapshot([]string{ToolIDWriteFile}, snap)
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(defs))
	}

	ws := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: ws})
	ctx = ContextWithSessionID(ctx, "session-1")
	ctx = ContextWithSettingsDB(ctx, db)

	raw, _ := json.Marshal(map[string]any{
		"filePath": "a.txt",
		"content":  "hello\n",
	})

	_, err = defs[0].Handler(ctx, raw)
	if err == nil {
		t.Fatalf("expected approval required error")
	}
	var reqErr *ApprovalRequiredError
	if !errors.As(err, &reqErr) {
		t.Fatalf("expected ApprovalRequiredError, got %T (%v)", err, err)
	}
	if reqErr.ApprovalID == "" {
		t.Fatalf("expected approval id")
	}

	if err := db.DenyToolApprovalForPrincipal(context.Background(), "alice", reqErr.ApprovalID, "no"); err != nil {
		t.Fatalf("deny: %v", err)
	}

	_, err = defs[0].Handler(ctx, raw)
	if err == nil {
		t.Fatalf("expected approval denied error")
	}
	var denyErr *ApprovalDeniedError
	if !errors.As(err, &denyErr) {
		t.Fatalf("expected ApprovalDeniedError, got %T (%v)", err, err)
	}
	if _, statErr := os.Stat(filepath.Join(ws, "a.txt")); statErr == nil {
		t.Fatalf("expected file not to be written when denied")
	}
}
