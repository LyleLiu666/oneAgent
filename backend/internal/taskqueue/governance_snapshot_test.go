package taskqueue

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTaskRunner_GovernanceSnapshot_ReasonCodes(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "tasks"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	wsA := filepath.Join(base, "wsA")
	wsB := filepath.Join(base, "wsB")
	wsC := filepath.Join(base, "wsC")
	mustMkdir(t, wsA)
	mustMkdir(t, wsB)
	mustMkdir(t, wsC)

	_, err = store.UpdateGovernance(func(g *QueueGovernance) error {
		g.Global.MaxRunningWorkspaces = 1
		g.Workspaces[wsA] = WorkspacePolicy{Paused: false, Priority: 1}
		g.Workspaces[wsB] = WorkspacePolicy{Paused: false, Priority: 0}
		g.Workspaces[wsC] = WorkspacePolicy{Paused: true, Priority: 0}
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateGovernance: %v", err)
	}

	r := &TaskRunner{
		Store:             store,
		queues:            map[string][]string{wsA: {"tA"}, wsB: {"tB"}, wsC: {"tC"}},
		runningWorkspaces: map[string]bool{wsA: true},
		workspaceAges:     map[string]int{},
	}

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	snap := r.GovernanceSnapshot(now)

	if got := snap.Workspaces[wsA].ReasonCode; got != ScheduleReasonWorkspaceRunning {
		t.Fatalf("wsA reason_code=%q, want %q", got, ScheduleReasonWorkspaceRunning)
	}
	if got := snap.Workspaces[wsB].ReasonCode; got != ScheduleReasonGlobalCap {
		t.Fatalf("wsB reason_code=%q, want %q", got, ScheduleReasonGlobalCap)
	}
	if got := snap.Workspaces[wsC].ReasonCode; got != ScheduleReasonWorkspacePaused {
		t.Fatalf("wsC reason_code=%q, want %q", got, ScheduleReasonWorkspacePaused)
	}
	if snap.PausedWorkspaces != 1 {
		t.Fatalf("paused_workspaces=%d, want 1", snap.PausedWorkspaces)
	}
	if snap.DeferredWorkspaces != 2 {
		t.Fatalf("deferred_workspaces=%d, want 2", snap.DeferredWorkspaces)
	}
}

func TestTaskRunner_GovernanceSnapshot_PriorityPreemption(t *testing.T) {
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "tasks"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	wsHigh := filepath.Join(base, "wsHigh")
	wsLow := filepath.Join(base, "wsLow")
	mustMkdir(t, wsHigh)
	mustMkdir(t, wsLow)

	_, err = store.UpdateGovernance(func(g *QueueGovernance) error {
		g.Global.MaxRunningWorkspaces = 0
		g.Workspaces[wsHigh] = WorkspacePolicy{Paused: false, Priority: 2}
		g.Workspaces[wsLow] = WorkspacePolicy{Paused: false, Priority: 0}
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateGovernance: %v", err)
	}

	r := &TaskRunner{
		Store:             store,
		queues:            map[string][]string{wsHigh: {"tH"}, wsLow: {"tL"}},
		runningWorkspaces: map[string]bool{},
		workspaceAges:     map[string]int{},
	}

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	snap := r.GovernanceSnapshot(now)

	if got := snap.Workspaces[wsHigh].Decision; got != ScheduleDecisionPicked {
		t.Fatalf("wsHigh decision=%q, want %q", got, ScheduleDecisionPicked)
	}
	if got := snap.Workspaces[wsLow].Decision; got != ScheduleDecisionDeferred {
		t.Fatalf("wsLow decision=%q, want %q", got, ScheduleDecisionDeferred)
	}
	if got := snap.Workspaces[wsLow].ReasonCode; got != ScheduleReasonPriorityPreempted {
		t.Fatalf("wsLow reason_code=%q, want %q", got, ScheduleReasonPriorityPreempted)
	}
}
