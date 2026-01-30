package tool

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
)

func TestRequireToolApprovalIfNeeded_ReusesPendingApprovalForEquivalentJSON(t *testing.T) {
	db, err := settingsdb.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("open settingsdb: %v", err)
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

	ctx := context.Background()
	ctx = ContextWithPolicySnapshot(ctx, snap)
	ctx = ContextWithSettingsDB(ctx, db)
	ctx = ContextWithSessionID(ctx, "sess-1")

	rawA := json.RawMessage(`{"filePath":"a.txt","content":"hi","append":true}`)
	rawB := json.RawMessage(`{"content":"hi","append":true,"filePath":"a.txt"}`)

	err = requireToolApprovalIfNeeded(ctx, ToolIDWriteFile, rawA)
	var req1 *ApprovalRequiredError
	if !errors.As(err, &req1) {
		t.Fatalf("expected approval required, got %v", err)
	}
	if req1.ApprovalID == "" {
		t.Fatalf("expected approval_id")
	}

	err = requireToolApprovalIfNeeded(ctx, ToolIDWriteFile, rawB)
	var req2 *ApprovalRequiredError
	if !errors.As(err, &req2) {
		t.Fatalf("expected approval required, got %v", err)
	}
	if req2.ApprovalID != req1.ApprovalID {
		t.Fatalf("expected pending approval to be reused, got %q != %q", req2.ApprovalID, req1.ApprovalID)
	}

	if err := db.ApproveToolApproval(context.Background(), req1.ApprovalID, "ok"); err != nil {
		t.Fatalf("approve: %v", err)
	}

	if err := requireToolApprovalIfNeeded(ctx, ToolIDWriteFile, rawA); err != nil {
		t.Fatalf("expected approval to be consumed and allow tool, got %v", err)
	}

	err = requireToolApprovalIfNeeded(ctx, ToolIDWriteFile, rawA)
	var req3 *ApprovalRequiredError
	if !errors.As(err, &req3) {
		t.Fatalf("expected approval required again after consumption, got %v", err)
	}
	if req3.ApprovalID == "" || req3.ApprovalID == req1.ApprovalID {
		t.Fatalf("expected new approval_id after consumption, got %q", req3.ApprovalID)
	}
}
