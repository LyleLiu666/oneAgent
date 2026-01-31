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

func TestApprovalGate_CommandTools_HighRisk_Manual(t *testing.T) {
	db, err := settingsdb.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("open settingsdb: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.SetUserSetting(context.Background(), "alice", settingsdb.SettingKeyCommandApprovalMode, "manual"); err != nil {
		t.Fatalf("set command approval mode: %v", err)
	}

	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{
			{
				ID:     "allow-bash-high-risk-approval",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDBash,
				Constraints: permissions.Constraints{
					Approval:       "high_risk",
					CommandProfile: "dev",
					SandboxMode:    "native",
				},
			},
			{
				ID:     "allow-run-command-high-risk-approval",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDRunCommand,
				Constraints: permissions.Constraints{
					Approval:       "high_risk",
					CommandProfile: "dev",
					SandboxMode:    "native",
				},
			},
		},
	}
	snap := permissions.ResolveSnapshot("alice", policy, time.Now())

	ws := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: ws})
	ctx = ContextWithPolicySnapshot(ctx, snap)
	ctx = ContextWithSettingsDB(ctx, db)
	ctx = ContextWithSessionID(ctx, "sess-1")

	rawSafe := json.RawMessage(`{"command":"ls -la"}`)
	if err := requireToolApprovalIfNeeded(ctx, ToolIDBash, rawSafe); err != nil {
		t.Fatalf("expected safe command to bypass approval, got %v", err)
	}

	rawHigh := json.RawMessage(`{"command":"rm -rf a"}`)
	err = requireToolApprovalIfNeeded(ctx, ToolIDBash, rawHigh)
	var req1 *ApprovalRequiredError
	if !errors.As(err, &req1) {
		t.Fatalf("expected approval required, got %v", err)
	}
	if req1.ApprovalID == "" {
		t.Fatalf("expected approval_id")
	}

	if err := db.ApproveToolApproval(context.Background(), req1.ApprovalID, "ok"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := requireToolApprovalIfNeeded(ctx, ToolIDBash, rawHigh); err != nil {
		t.Fatalf("expected approval to be consumed and allow call, got %v", err)
	}

	// Consumed: requires a new approval.
	err = requireToolApprovalIfNeeded(ctx, ToolIDBash, rawHigh)
	var req2 *ApprovalRequiredError
	if !errors.As(err, &req2) {
		t.Fatalf("expected approval required again after consume, got %v", err)
	}
	if req2.ApprovalID == "" || req2.ApprovalID == req1.ApprovalID {
		t.Fatalf("expected new approval_id after consume, got %q", req2.ApprovalID)
	}

	// run_command poll/cancel should not trigger approval even if high-risk gating is enabled.
	rawPoll := json.RawMessage(`{"action":"poll","job_id":"job-1"}`)
	if err := requireToolApprovalIfNeeded(ctx, ToolIDRunCommand, rawPoll); err != nil {
		t.Fatalf("expected poll to bypass approval, got %v", err)
	}
	rawCancel := json.RawMessage(`{"action":"cancel","job_id":"job-1"}`)
	if err := requireToolApprovalIfNeeded(ctx, ToolIDRunCommand, rawCancel); err != nil {
		t.Fatalf("expected cancel to bypass approval, got %v", err)
	}
}

func TestApprovalGate_CommandTools_HighRisk_Auto(t *testing.T) {
	db, err := settingsdb.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("open settingsdb: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Default is auto; set explicitly for clarity.
	if err := db.SetUserSetting(context.Background(), "alice", settingsdb.SettingKeyCommandApprovalMode, "auto"); err != nil {
		t.Fatalf("set command approval mode: %v", err)
	}

	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{
			{
				ID:     "allow-bash-high-risk-approval",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDBash,
				Constraints: permissions.Constraints{
					Approval:       "high_risk",
					CommandProfile: "dev",
					SandboxMode:    "native",
				},
			},
		},
	}
	snap := permissions.ResolveSnapshot("alice", policy, time.Now())

	ws := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: ws})
	ctx = ContextWithPolicySnapshot(ctx, snap)
	ctx = ContextWithSettingsDB(ctx, db)
	ctx = ContextWithSessionID(ctx, "sess-1")

	rawHigh := json.RawMessage(`{"command":"rm -rf a"}`)
	if err := requireToolApprovalIfNeeded(ctx, ToolIDBash, rawHigh); err != nil {
		t.Fatalf("expected auto mode to allow, got %v", err)
	}

	argsHash := toolArgsHash(rawHigh)
	decision, ok, err := db.LookupLatestToolApprovalDecision(ctx, "alice", "sess-1", ToolIDBash, argsHash)
	if err != nil {
		t.Fatalf("lookup decision: %v", err)
	}
	if !ok {
		t.Fatalf("expected approval record")
	}
	if decision.Status != "consumed" {
		t.Fatalf("expected status=consumed, got %q", decision.Status)
	}
	if decision.Reason != "auto-approved" {
		t.Fatalf("expected reason=auto-approved, got %q", decision.Reason)
	}
}

func TestApprovalGate_CommandTools_HighRisk_Manual_SystemInstallCreatesApproval(t *testing.T) {
	db, err := settingsdb.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("open settingsdb: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.SetUserSetting(context.Background(), "alice", settingsdb.SettingKeyCommandApprovalMode, "manual"); err != nil {
		t.Fatalf("set command approval mode: %v", err)
	}

	policy := permissions.Policy{
		ID:            "p1",
		DefaultEffect: permissions.EffectDeny,
		Rules: []permissions.Rule{
			{
				ID:     "allow-bash-high-risk-approval",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDBash,
				Constraints: permissions.Constraints{
					Approval:       "high_risk",
					CommandProfile: "dev",
					SandboxMode:    "native",
				},
			},
		},
	}
	snap := permissions.ResolveSnapshot("alice", policy, time.Now())

	ws := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: ws})
	ctx = ContextWithPolicySnapshot(ctx, snap)
	ctx = ContextWithSettingsDB(ctx, db)
	ctx = ContextWithSessionID(ctx, "sess-1")

	rawHigh := json.RawMessage(`{"command":"brew install pango"}`)
	err = requireToolApprovalIfNeeded(ctx, ToolIDBash, rawHigh)
	var req *ApprovalRequiredError
	if !errors.As(err, &req) {
		t.Fatalf("expected approval required, got %v", err)
	}
	if req.ApprovalID == "" {
		t.Fatalf("expected approval_id")
	}
}

func TestApprovalGate_CommandTools_HighRisk_SkipsApprovalWhenCommandWouldBeDenied(t *testing.T) {
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
				ID:     "allow-bash-high-risk-approval",
				Effect: permissions.EffectAllow,
				ToolID: ToolIDBash,
				Constraints: permissions.Constraints{
					Approval:       "high_risk",
					CommandProfile: "dev",
					SandboxMode:    "none",
				},
			},
		},
	}
	snap := permissions.ResolveSnapshot("alice", policy, time.Now())

	ws := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: ws})
	ctx = ContextWithPolicySnapshot(ctx, snap)
	ctx = ContextWithSettingsDB(ctx, db)
	ctx = ContextWithSessionID(ctx, "sess-1")

	rawHigh := json.RawMessage(`{"command":"rm -rf a"}`)
	if err := requireToolApprovalIfNeeded(ctx, ToolIDBash, rawHigh); err != nil {
		t.Fatalf("expected high-risk command to bypass approval when it will be denied anyway, got %v", err)
	}

	argsHash := toolArgsHash(rawHigh)
	if _, ok, err := db.LookupLatestToolApprovalDecision(ctx, "alice", "sess-1", ToolIDBash, argsHash); err != nil {
		t.Fatalf("lookup decision: %v", err)
	} else if ok {
		t.Fatalf("expected no approval record when command would be denied")
	}
}
