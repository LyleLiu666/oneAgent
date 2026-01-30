package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type NodeExecutor interface {
	ExecuteNode(ctx context.Context, run WorkflowRun, node Node) (ArtifactManifest, error)
}

type GateEvaluator interface {
	EvaluateHardGate(ctx context.Context, run WorkflowRun, node Node, manifest ArtifactManifest) (reportPath string, err error)
	EvaluateSoftGate(ctx context.Context, run WorkflowRun, node Node, manifest ArtifactManifest) (reportPath string, err error)
}

type RunOptions struct {
	Concurrency int
	Gates       GateEvaluator
}

func (s *Store) RunWorkflow(ctx context.Context, workspaceRoot string, workflowID string, runID string, exec NodeExecutor, opts RunOptions) (WorkflowRun, error) {
	if s == nil {
		return WorkflowRun{}, errors.New("store is nil")
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	workflowID = strings.TrimSpace(workflowID)
	runID = strings.TrimSpace(runID)
	if workspaceRoot == "" || workflowID == "" || runID == "" {
		return WorkflowRun{}, errors.New("workspace_root, workflow_id, run_id are required")
	}
	if exec == nil {
		return WorkflowRun{}, errors.New("executor is required")
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 2
	}
	if concurrency > 16 {
		concurrency = 16
	}

	// Mark run running best-effort.
	run, err := s.UpdateRun(ctx, workspaceRoot, workflowID, runID, func(cur *WorkflowRun) error {
		if cur.Status == RunStatusSucceeded || cur.Status == RunStatusFailed || cur.Status == RunStatusCanceled {
			return nil
		}
		cur.Status = RunStatusRunning
		now := time.Now().UTC()
		if cur.StartedAt.IsZero() {
			cur.StartedAt = now
		}
		return nil
	})
	if err != nil {
		return WorkflowRun{}, err
	}
	if run.Status != RunStatusRunning {
		return run, nil
	}

	type scheduleState struct {
		mu  sync.Mutex
		run WorkflowRun
	}
	state := &scheduleState{run: run}

	// Build graph indexes.
	nodeByID := map[string]Node{}
	for _, n := range run.GraphSnapshot.Nodes {
		id := strings.TrimSpace(n.NodeID)
		if id == "" {
			continue
		}
		nodeByID[id] = n
		state.mu.Lock()
		if state.run.NodeRuns == nil {
			state.run.NodeRuns = map[string]NodeRun{}
		}
		if _, ok := state.run.NodeRuns[id]; !ok {
			state.run.NodeRuns[id] = NodeRun{NodeID: id, Status: NodeStatusQueued}
		}
		state.mu.Unlock()
	}

	deps := map[string]map[string]bool{}
	for _, e := range run.GraphSnapshot.Edges {
		from := strings.TrimSpace(e.From)
		to := strings.TrimSpace(e.To)
		if from == "" || to == "" || from == to {
			continue
		}
		if _, ok := nodeByID[from]; !ok {
			continue
		}
		if _, ok := nodeByID[to]; !ok {
			continue
		}
		if deps[to] == nil {
			deps[to] = map[string]bool{}
		}
		deps[to][from] = true
	}

	// Best-effort cycle detection (Kahn).
	if hasCycleV1(nodeByID, deps) {
		run, _ = s.UpdateRun(ctx, workspaceRoot, workflowID, runID, func(cur *WorkflowRun) error {
			cur.Status = RunStatusFailed
			cur.Error = "cycle detected"
			cur.FinishedAt = time.Now().UTC()
			return nil
		})
		return run, fmt.Errorf("workflow cycle detected")
	}

	ready := make(chan string, len(nodeByID))
	done := make(chan struct{})
	var wg sync.WaitGroup

	markReady := func() {
		for id := range nodeByID {
			state.mu.Lock()
			nr := state.run.NodeRuns[id]
			if nr.Status != NodeStatusQueued {
				state.mu.Unlock()
				continue
			}
			ok := true
			for dep := range deps[id] {
				st := state.run.NodeRuns[dep].Status
				if st != NodeStatusSucceeded && st != NodeStatusSkipped {
					ok = false
					break
				}
			}
			state.mu.Unlock()
			if ok {
				select {
				case ready <- id:
				default:
				}
			}
		}
	}
	markReady()

	var mu sync.Mutex
	inFlight := map[string]bool{}
	enqueueReady := func() {
		// Re-evaluate and push newly-ready nodes.
		for id := range nodeByID {
			mu.Lock()
			already := inFlight[id]
			mu.Unlock()
			if already {
				continue
			}
			state.mu.Lock()
			nr := state.run.NodeRuns[id]
			if nr.Status != NodeStatusQueued {
				state.mu.Unlock()
				continue
			}
			ok := true
			for dep := range deps[id] {
				st := state.run.NodeRuns[dep].Status
				if st != NodeStatusSucceeded && st != NodeStatusSkipped {
					ok = false
					break
				}
			}
			state.mu.Unlock()
			if !ok {
				continue
			}
			select {
			case ready <- id:
			default:
			}
		}
	}

	worker := func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case nodeID := <-ready:
				nodeID = strings.TrimSpace(nodeID)
				if nodeID == "" {
					continue
				}
				mu.Lock()
				if inFlight[nodeID] {
					mu.Unlock()
					continue
				}
				inFlight[nodeID] = true
				mu.Unlock()

				// Transition to running.
				runningRun, err := s.UpdateRun(ctx, workspaceRoot, workflowID, runID, func(cur *WorkflowRun) error {
					nr := cur.NodeRuns[nodeID]
					if nr.Status != NodeStatusQueued {
						return nil
					}
					nr.Status = NodeStatusRunning
					nr.StartedAt = time.Now().UTC()
					cur.NodeRuns[nodeID] = nr
					return nil
				})
				if err != nil {
					return
				}
				state.mu.Lock()
				state.run = runningRun
				state.mu.Unlock()

				n := nodeByID[nodeID]
				manifest, execErr := exec.ExecuteNode(ctx, runningRun, n)

				hardReport := ""
				softReport := ""
				if execErr == nil && opts.Gates != nil {
					if p, err := opts.Gates.EvaluateHardGate(ctx, runningRun, n, manifest); err == nil {
						hardReport = strings.TrimSpace(p)
					}
					if p, err := opts.Gates.EvaluateSoftGate(ctx, runningRun, n, manifest); err == nil {
						softReport = strings.TrimSpace(p)
					}
				}

				// Persist node result.
				updatedRun, err := s.UpdateRun(ctx, workspaceRoot, workflowID, runID, func(cur *WorkflowRun) error {
					nr := cur.NodeRuns[nodeID]
					if nr.Status != NodeStatusRunning {
						return nil
					}
					nr.FinishedAt = time.Now().UTC()
					if execErr != nil {
						nr.Status = NodeStatusFailed
						nr.Error = execErr.Error()
						cur.Status = RunStatusFailed
						cur.Error = execErr.Error()
						cur.FinishedAt = nr.FinishedAt
					} else {
						nr.Status = NodeStatusSucceeded
						nr.Artifacts = manifest
						nr.HardGateReportPath = hardReport
						nr.SoftGateReportPath = softReport
					}
					cur.NodeRuns[nodeID] = nr
					return nil
				})
				if err != nil {
					return
				}
				state.mu.Lock()
				state.run = updatedRun
				state.mu.Unlock()

				mu.Lock()
				delete(inFlight, nodeID)
				mu.Unlock()

				if updatedRun.Status == RunStatusFailed {
					closeOnce(done)
					return
				}

				enqueueReady()
			}
		}
	}

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go worker()
	}

	// Wait for completion.
waitLoop:
	for {
		state.mu.Lock()
		cur := state.run
		state.mu.Unlock()
		if cur.Status == RunStatusFailed || cur.Status == RunStatusCanceled {
			break
		}
		allDone := true
		for id := range nodeByID {
			st := cur.NodeRuns[id].Status
			if st == NodeStatusQueued || st == NodeStatusRunning {
				allDone = false
				break
			}
		}
		if allDone {
			break
		}
		select {
		case <-ctx.Done():
			break waitLoop
		case <-time.After(10 * time.Millisecond):
			// poll run file state best-effort
			polled, err := s.GetRun(ctx, workspaceRoot, workflowID, runID)
			if err == nil {
				state.mu.Lock()
				state.run = polled
				state.mu.Unlock()
			}
		}
	}

	closeOnce(done)
	wg.Wait()

	// Finalize status if succeeded.
	run, _ = s.UpdateRun(ctx, workspaceRoot, workflowID, runID, func(cur *WorkflowRun) error {
		if cur.Status != RunStatusRunning {
			return nil
		}
		for _, nr := range cur.NodeRuns {
			if nr.Status == NodeStatusQueued || nr.Status == NodeStatusRunning {
				return nil
			}
			if nr.Status == NodeStatusFailed {
				cur.Status = RunStatusFailed
				cur.FinishedAt = time.Now().UTC()
				return nil
			}
		}
		cur.Status = RunStatusSucceeded
		cur.FinishedAt = time.Now().UTC()
		return nil
	})

	return run, nil
}

func hasCycleV1(nodes map[string]Node, deps map[string]map[string]bool) bool {
	inDegree := map[string]int{}
	for id := range nodes {
		inDegree[id] = 0
	}
	for to, ds := range deps {
		for dep := range ds {
			if _, ok := nodes[to]; !ok {
				continue
			}
			if _, ok := nodes[dep]; !ok {
				continue
			}
			inDegree[to]++
		}
	}
	queue := make([]string, 0, len(nodes))
	for id, d := range inDegree {
		if d == 0 {
			queue = append(queue, id)
		}
	}
	seen := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		seen++
		for to := range depsByFromV1(deps, id) {
			inDegree[to]--
			if inDegree[to] == 0 {
				queue = append(queue, to)
			}
		}
	}
	return seen != len(nodes)
}

func depsByFromV1(deps map[string]map[string]bool, from string) map[string]bool {
	out := map[string]bool{}
	for to, ds := range deps {
		if ds[from] {
			out[to] = true
		}
	}
	return out
}

func closeOnce(ch chan struct{}) {
	defer func() { _ = recover() }()
	close(ch)
}
