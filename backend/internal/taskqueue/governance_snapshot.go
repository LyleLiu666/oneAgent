package taskqueue

import "time"

const (
	ScheduleDecisionPicked   = "picked"
	ScheduleDecisionDeferred = "deferred"
	ScheduleDecisionIdle     = "idle"
)

const (
	ScheduleReasonPicked             = "picked"
	ScheduleReasonNoQueuedTasks      = "no_queued_tasks"
	ScheduleReasonWorkspacePaused    = "workspace_paused"
	ScheduleReasonWorkspaceRunning   = "workspace_running"
	ScheduleReasonGlobalCap          = "global_cap"
	ScheduleReasonPriorityPreempted  = "priority_preempted"
	ScheduleReasonGovernanceNotReady = "governance_unavailable"
)

type WorkspaceGovernanceSnapshot struct {
	QueuedTasks       int    `json:"queued_tasks,omitempty"`
	Running           bool   `json:"running,omitempty"`
	Paused            bool   `json:"paused,omitempty"`
	Priority          int    `json:"priority,omitempty"`
	Age               int    `json:"age,omitempty"`
	EffectivePriority int    `json:"effective_priority,omitempty"`
	Decision          string `json:"decision,omitempty"`
	ReasonCode        string `json:"reason_code,omitempty"`
	SelectedTaskID    string `json:"selected_task_id,omitempty"`
}

type QueueGovernanceSnapshot struct {
	TS time.Time `json:"ts"`

	Global QueueGlobalPolicy `json:"global"`

	RunningWorkspaces  []string                               `json:"running_workspaces,omitempty"`
	DeferredWorkspaces int                                    `json:"deferred_workspaces,omitempty"`
	PausedWorkspaces   int                                    `json:"paused_workspaces,omitempty"`
	Workspaces         map[string]WorkspaceGovernanceSnapshot `json:"workspaces,omitempty"`
}

func (r *TaskRunner) GovernanceSnapshot(now time.Time) QueueGovernanceSnapshot {
	if r == nil || r.Store == nil {
		return QueueGovernanceSnapshot{TS: now.UTC()}
	}

	g, _ := r.Store.GetGovernance()

	r.mu.Lock()
	defer r.mu.Unlock()

	snap := QueueGovernanceSnapshot{
		TS:         now.UTC(),
		Global:     g.Global,
		Workspaces: make(map[string]WorkspaceGovernanceSnapshot),
	}

	wsSet := make(map[string]struct{})
	for ws, running := range r.runningWorkspaces {
		if running {
			wsSet[ws] = struct{}{}
			snap.RunningWorkspaces = append(snap.RunningWorkspaces, ws)
		}
	}
	for ws := range r.queues {
		wsSet[ws] = struct{}{}
	}
	for ws := range g.Workspaces {
		wsSet[ws] = struct{}{}
	}

	maxWS := g.Global.MaxRunningWorkspaces
	runningCount := 0
	for _, v := range r.runningWorkspaces {
		if v {
			runningCount++
		}
	}
	capReached := maxWS > 0 && runningCount >= maxWS
	bestWS := ""
	bestScore := -1 << 30

	for ws := range wsSet {
		q := r.queues[ws]
		queued := len(q)
		p := g.Workspaces[ws]

		paused := p.Paused
		running := r.runningWorkspaces[ws]
		age := r.workspaceAges[ws]
		effective := p.Priority + age

		wsSnap := WorkspaceGovernanceSnapshot{
			QueuedTasks:       queued,
			Running:           running,
			Paused:            paused,
			Priority:          p.Priority,
			Age:               age,
			EffectivePriority: effective,
		}

		switch {
		case queued == 0:
			wsSnap.Decision = ScheduleDecisionIdle
			wsSnap.ReasonCode = ScheduleReasonNoQueuedTasks
		case paused:
			wsSnap.Decision = ScheduleDecisionDeferred
			wsSnap.ReasonCode = ScheduleReasonWorkspacePaused
		case running:
			wsSnap.Decision = ScheduleDecisionDeferred
			wsSnap.ReasonCode = ScheduleReasonWorkspaceRunning
		case capReached:
			wsSnap.Decision = ScheduleDecisionDeferred
			wsSnap.ReasonCode = ScheduleReasonGlobalCap
		default:
			if bestWS == "" || effective > bestScore || (effective == bestScore && ws < bestWS) {
				bestWS = ws
				bestScore = effective
			}
			// Default to deferred; best workspace will be overwritten to picked below.
			wsSnap.Decision = ScheduleDecisionDeferred
			wsSnap.ReasonCode = ScheduleReasonPriorityPreempted
		}

		snap.Workspaces[ws] = wsSnap
	}

	if bestWS != "" {
		wsSnap := snap.Workspaces[bestWS]
		wsSnap.Decision = ScheduleDecisionPicked
		wsSnap.ReasonCode = ScheduleReasonPicked
		if q := r.queues[bestWS]; len(q) > 0 {
			wsSnap.SelectedTaskID = q[0]
		}
		snap.Workspaces[bestWS] = wsSnap
	}

	for _, wsSnap := range snap.Workspaces {
		if wsSnap.Decision == ScheduleDecisionDeferred && wsSnap.QueuedTasks > 0 && !wsSnap.Paused {
			// Count deferred workspaces with queued tasks (paused is tracked separately).
			snap.DeferredWorkspaces++
		}
	}

	for _, p := range g.Workspaces {
		if p.Paused {
			snap.PausedWorkspaces++
		}
	}

	return snap
}
