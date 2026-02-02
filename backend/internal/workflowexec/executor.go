package workflowexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/gitutil"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/projectcfg"
	"github.com/liu_y/oneAgent/backend/internal/prompt"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/subagent"
	"github.com/liu_y/oneAgent/backend/internal/tool"
	"github.com/liu_y/oneAgent/backend/internal/workflow"
)

type ClientResolver interface {
	ResolveClient(ctx context.Context, principalID, modelID string) (llm.Client, string, error)
}

type Executor struct {
	Store   *workflow.Store
	Runtime *runtime.Runtime

	Resolver ClientResolver

	DefaultPrincipalID string

	WorkspaceRoot   string
	ExecutionRoot   string
	cleanupWorktree func()
}

type PrepareOptions struct {
	WorkspaceRoot string
	WorkflowID    string
	RunID         string
}

func Prepare(ctx context.Context, rt *runtime.Runtime, store *workflow.Store, defaultPrincipalID string, opts PrepareOptions) (*Executor, func(), error) {
	if rt == nil || rt.Layout == nil {
		return nil, nil, errors.New("runtime is required")
	}
	if store == nil {
		return nil, nil, errors.New("workflow store is required")
	}

	wsRoot := strings.TrimSpace(opts.WorkspaceRoot)
	if wsRoot == "" {
		return nil, nil, errors.New("workspace_root is required")
	}

	workflowID := strings.TrimSpace(opts.WorkflowID)
	runID := strings.TrimSpace(opts.RunID)
	if workflowID == "" || runID == "" {
		return nil, nil, errors.New("workflow_id and run_id are required")
	}

	execRoot := wsRoot
	cleanup := func() {}

	projectCfg, found, err := projectcfg.Load(wsRoot)
	if err != nil {
		return nil, nil, err
	}
	if found && projectCfg.AttemptExecutionMode == "worktree" {
		base, err := gitutil.ResolveGitBase(ctx, wsRoot)
		if err != nil {
			return nil, nil, err
		}
		runRoot := store.RunRoot(wsRoot, workflowID, runID)
		if runRoot == "" {
			return nil, nil, errors.New("run root is required")
		}
		worktreeRoot := filepath.Join(runRoot, "execution", "worktree")
		if err := gitutil.CreateWorktree(ctx, wsRoot, worktreeRoot, base.BaseCommitSHA); err != nil {
			return nil, nil, err
		}
		execRoot = worktreeRoot
		cleanup = func() {
			if strings.TrimSpace(os.Getenv("ONEAGENT_KEEP_WORKTREES")) == "1" {
				return
			}
			_ = gitutil.RemoveWorktree(context.Background(), wsRoot, worktreeRoot)
		}
	}

	exec := &Executor{
		Store:              store,
		Runtime:            rt,
		DefaultPrincipalID: strings.TrimSpace(defaultPrincipalID),
		WorkspaceRoot:      wsRoot,
		ExecutionRoot:      execRoot,
		cleanupWorktree:    cleanup,
	}
	return exec, cleanup, nil
}

func (e *Executor) ExecuteNode(ctx context.Context, run workflow.WorkflowRun, node workflow.Node) (workflow.ArtifactManifest, error) {
	if e == nil || e.Store == nil || e.Runtime == nil {
		return workflow.ArtifactManifest{}, errors.New("executor is not initialized")
	}
	nodeID := strings.TrimSpace(node.NodeID)
	if nodeID == "" {
		return workflow.ArtifactManifest{}, errors.New("node_id is required")
	}

	nodeRoot, err := e.Store.EnsureNodeRoot(run.WorkspaceRoot, run.WorkflowID, run.RunID, nodeID)
	if err != nil {
		return workflow.ArtifactManifest{}, err
	}

	inputs, contextSummary := buildInputs(run, nodeID)
	inputsPath := filepath.Join(nodeRoot, "inputs.json")
	if err := writeJSONFile(inputsPath, inputs, 0o600); err != nil {
		return workflow.ArtifactManifest{}, err
	}

	// Resolve principal/model/skills (best-effort).
	principalID := strings.TrimSpace(node.PrincipalID)
	if principalID == "" {
		principalID = strings.TrimSpace(e.DefaultPrincipalID)
	}
	if principalID == "" {
		principalID = "local"
	}

	policySnap, err := e.Runtime.ResolveToolPolicySnapshot(ctx, principalID)
	if err != nil {
		return workflow.ArtifactManifest{}, err
	}

	resolver := e.Resolver
	if resolver == nil {
		resolver = settingsResolver{Settings: e.Runtime.Settings}
	}
	client, _, err := resolver.ResolveClient(ctx, principalID, strings.TrimSpace(node.ModelID))
	if err != nil {
		return workflow.ArtifactManifest{}, err
	}

	defs, _, systemPrompt, handlers, err := mountTools(policySnap)
	if err != nil {
		return workflow.ArtifactManifest{}, err
	}

	toolCtx := ctx
	toolCtx = tool.ContextWithUserID(toolCtx, principalID)
	toolCtx = tool.ContextWithPolicySnapshot(toolCtx, policySnap)
	toolCtx = tool.ContextWithSettingsDB(toolCtx, e.Runtime.Settings)
	toolCtx = tool.ContextWithSkillManager(toolCtx, e.Runtime.Skills)
	toolCtx = tool.ContextWithWorkspace(toolCtx, tool.WorkspaceConfig{Enabled: true, Root: e.ExecutionRoot})
	toolCtx = tool.ContextWithOCC(toolCtx, strings.TrimSpace(os.Getenv("ONEAGENT_DISABLE_OCC")) != "1")

	task := strings.TrimSpace(node.Prompt)
	if task == "" {
		task = strings.TrimSpace(node.Title)
	}
	if task == "" {
		task = nodeID
	}

	skillsSummary := ""

	result, runErr := subagent.Run(toolCtx, subagent.RunRequest{
		ParentSessionID:   run.RunID,
		UserID:            principalID,
		SystemPrompt:      systemPrompt,
		Client:            client,
		Tools:             tool.ToolsForLLM(defs),
		Handlers:          handlers,
		WorkspaceRoot:     e.ExecutionRoot,
		WriteScope:        nil,
		LogsBaseDir:       e.Runtime.Layout.SubagentLogsDir,
		Task:              task,
		ContextSummary:    contextSummary,
		SkillsSummary:     skillsSummary,
		MaxSteps:          200,
		MaxRuntimeSeconds: 3600,
	})

	// Always attempt to export evidence into NodeRoot, even on failure.
	tracePath := filepath.Join(nodeRoot, "trace.jsonl")
	if err := copyFileBestEffort(strings.TrimSpace(result.TraceLogPath), tracePath); err != nil {
		_ = os.WriteFile(tracePath, []byte("{\"type\":\"missing_trace\"}\n"), 0o600)
	}

	ledgerPath := filepath.Join(nodeRoot, "LEDGER.md")
	findingsPath := filepath.Join(nodeRoot, "FINDINGS.md")

	subagentFindingsText := ""
	if strings.TrimSpace(result.FindingsPath) != "" {
		if b, err := os.ReadFile(result.FindingsPath); err == nil {
			subagentFindingsText = string(b)
		}
	}
	ledgerSection := extractMarkdownSection(subagentFindingsText, "## 流水账")
	findingsSection := extractMarkdownSection(subagentFindingsText, "## Findings")
	changedFiles := extractChangedFilesSection(subagentFindingsText)

	exported, exportNotes := exportDeliverables(e.ExecutionRoot, filepath.Join(nodeRoot, "deliverables"), changedFiles)

	diff := writeChangedFilesOnly(nodeRoot, e.ExecutionRoot, changedFiles)
	if gitutil.IsGitWorkspace(ctx, e.ExecutionRoot) {
		if got, err := gitutil.GenerateDiffArtifacts(ctx, e.ExecutionRoot, "", nodeRoot); err == nil {
			diff = got
		}
	}

	writeLedgerAndFindings(ledgerPath, findingsPath, ledgerSection, findingsSection, inputs, diff, exported, exportNotes)

	manifestPath := filepath.Join(nodeRoot, "artifacts.json")
	manifest := buildManifest(inputsPath, ledgerPath, findingsPath, tracePath, manifestPath, diff, exported)
	_ = writeJSONFile(manifestPath, artifactManifestFile{
		SchemaVersion: 1,
		NodeID:        nodeID,
		RunID:         run.RunID,
		Artifacts:     manifest.Artifacts,
	}, 0o600)

	if runErr != nil {
		return manifest, runErr
	}
	return manifest, nil
}

type artifactManifestFile struct {
	SchemaVersion int                 `json:"schema_version"`
	NodeID        string              `json:"node_id"`
	RunID         string              `json:"run_id"`
	Artifacts     []workflow.Artifact `json:"artifacts"`
}

type nodeInputsFile struct {
	SchemaVersion int             `json:"schema_version"`
	NodeID        string          `json:"node_id"`
	RunID         string          `json:"run_id"`
	Upstreams     []upstreamInput `json:"upstreams,omitempty"`
}

type upstreamInput struct {
	NodeID       string              `json:"node_id"`
	ManifestPath string              `json:"manifest_path,omitempty"`
	Artifacts    []workflow.Artifact `json:"artifacts,omitempty"`
}

func buildInputs(run workflow.WorkflowRun, nodeID string) (nodeInputsFile, string) {
	nodeID = strings.TrimSpace(nodeID)
	upstreamSet := make(map[string]struct{})
	for _, e := range run.GraphSnapshot.Edges {
		if strings.TrimSpace(e.To) != nodeID {
			continue
		}
		if from := strings.TrimSpace(e.From); from != "" && from != nodeID {
			upstreamSet[from] = struct{}{}
		}
	}
	upstreamIDs := make([]string, 0, len(upstreamSet))
	for id := range upstreamSet {
		upstreamIDs = append(upstreamIDs, id)
	}
	sort.Strings(upstreamIDs)

	upstreams := make([]upstreamInput, 0, len(upstreamIDs))
	var b strings.Builder
	if len(upstreamIDs) > 0 {
		b.WriteString("## Upstream inputs (paths only)\n")
	}

	for _, upID := range upstreamIDs {
		nr, ok := run.NodeRuns[upID]
		if !ok || len(nr.Artifacts.Artifacts) == 0 {
			upstreams = append(upstreams, upstreamInput{NodeID: upID})
			continue
		}
		artifacts := make([]workflow.Artifact, 0, len(nr.Artifacts.Artifacts))
		manifestPath := ""
		for _, a := range nr.Artifacts.Artifacts {
			artifacts = append(artifacts, a)
			if strings.TrimSpace(a.Kind) == "manifest" && strings.TrimSpace(a.Path) != "" {
				manifestPath = strings.TrimSpace(a.Path)
			}
		}

		upstreams = append(upstreams, upstreamInput{
			NodeID:       upID,
			ManifestPath: manifestPath,
			Artifacts:    artifacts,
		})

		b.WriteString("- ")
		b.WriteString(upID)
		b.WriteString("\n")
		if manifestPath != "" {
			b.WriteString("  - manifest: ")
			b.WriteString(manifestPath)
			b.WriteString("\n")
		}
		for _, a := range artifacts {
			if strings.TrimSpace(a.Path) == "" {
				continue
			}
			b.WriteString("  - ")
			if strings.TrimSpace(a.Kind) != "" {
				b.WriteString(a.Kind)
				b.WriteString(": ")
			}
			b.WriteString(a.Path)
			b.WriteString("\n")
		}
	}

	return nodeInputsFile{
		SchemaVersion: 1,
		NodeID:        nodeID,
		RunID:         run.RunID,
		Upstreams:     upstreams,
	}, strings.TrimSpace(b.String())
}

func mountTools(policySnap permissions.Snapshot) ([]tool.Definition, []string, string, map[string]subagent.ToolHandler, error) {
	toolIDs := make([]string, 0, 32)
	for _, def := range tool.All() {
		if def.ID == tool.ToolIDSubagent {
			continue
		}
		toolIDs = append(toolIDs, def.ID)
	}

	defs, err := tool.MountWithSnapshot(toolIDs, policySnap)
	if err != nil {
		return nil, nil, "", nil, err
	}
	toolNames := make([]string, 0, len(defs))
	handlers := make(map[string]subagent.ToolHandler, len(defs))
	for _, def := range defs {
		toolNames = append(toolNames, def.Spec.Function.Name)
		handlers[def.Spec.Function.Name] = subagent.ToolHandler(def.Handler)
	}

	assembled, err := prompt.AssembleStablePrefix(prompt.AssembleInput{ToolNames: toolNames})
	if err != nil {
		return nil, nil, "", nil, err
	}
	return defs, toolNames, assembled.StablePrefix, handlers, nil
}

type settingsResolver struct {
	Settings *settingsdb.DB
}

func (r settingsResolver) ResolveClient(ctx context.Context, principalID, modelID string) (llm.Client, string, error) {
	if r.Settings == nil {
		return nil, "", errors.New("settings db is required")
	}

	modelID = strings.TrimSpace(modelID)
	var m settingsdb.Model
	if modelID != "" {
		got, err := r.Settings.GetModel(ctx, principalID, modelID)
		if err != nil {
			return nil, "", err
		}
		m = got
	} else {
		models, err := r.Settings.ListModels(ctx, principalID, "")
		if err != nil {
			return nil, "", err
		}
		for _, candidate := range models {
			if candidate.IsDefault {
				m = candidate
				break
			}
		}
		if m.ID == "" {
			return nil, "", errors.New("no LLM model configured")
		}
	}

	provider, err := r.Settings.GetProvider(ctx, principalID, m.ProviderID)
	if err != nil {
		return nil, "", err
	}

	if strings.TrimSpace(provider.BaseURL) == "" || strings.TrimSpace(provider.APIKey) == "" {
		return nil, "", errors.New("provider base_url or api_key is missing")
	}

	client, err := llm.NewClientForProvider(llm.ProviderConfig{
		ProviderType: provider.ProviderType,
		Endpoint:     provider.BaseURL,
		APIKey:       provider.APIKey,
		Model:        m.Model,
	})
	if err != nil {
		return nil, "", err
	}
	return client, m.Model, nil
}

func writeJSONFile(path string, v any, mode os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), mode)
}

func copyFileBestEffort(src, dst string) error {
	src = strings.TrimSpace(src)
	dst = strings.TrimSpace(dst)
	if src == "" || dst == "" {
		return errors.New("src and dst are required")
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func extractMarkdownSection(text, header string) string {
	text = strings.TrimSpace(text)
	header = strings.TrimSpace(header)
	if text == "" || header == "" {
		return ""
	}
	idx := strings.Index(text, header)
	if idx == -1 {
		return ""
	}
	section := text[idx:]
	if next := strings.Index(section[len(header):], "\n## "); next != -1 {
		section = section[:len(header)+next+1]
	}
	return strings.TrimSpace(section)
}

func extractChangedFilesSection(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	idx := strings.Index(text, "## 变更文件")
	if idx == -1 {
		return nil
	}
	section := text[idx:]
	if next := strings.Index(section[len("## 变更文件"):], "\n## "); next != -1 {
		section = section[:len("## 变更文件")+next+1]
	}
	lines := strings.Split(section, "\n")
	out := make([]string, 0, 8)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if item == "" || strings.Contains(item, "（无）") || strings.Contains(item, "(none)") {
			continue
		}
		out = append(out, item)
	}
	uniq := make(map[string]struct{}, len(out))
	for _, p := range out {
		uniq[p] = struct{}{}
	}
	out = out[:0]
	for p := range uniq {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func exportDeliverables(execRoot, dstRoot string, relPaths []string) ([]string, []string) {
	execRoot = strings.TrimSpace(execRoot)
	dstRoot = strings.TrimSpace(dstRoot)
	if execRoot == "" || dstRoot == "" {
		return nil, []string{"export skipped: missing roots"}
	}
	exported := make([]string, 0, len(relPaths))
	notes := make([]string, 0, 4)

	for _, raw := range relPaths {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if filepath.IsAbs(raw) {
			if rel, err := filepath.Rel(execRoot, raw); err == nil && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && rel != "." {
				raw = rel
			} else {
				notes = append(notes, fmt.Sprintf("skip outside exec root: %s", raw))
				continue
			}
		}

		clean := filepath.Clean(raw)
		if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			notes = append(notes, fmt.Sprintf("skip invalid path: %s", raw))
			continue
		}

		src := filepath.Join(execRoot, clean)
		st, err := os.Stat(src)
		if err != nil {
			notes = append(notes, fmt.Sprintf("missing: %s", clean))
			continue
		}

		dst := filepath.Join(dstRoot, clean)
		if st.IsDir() {
			if err := copyDir(src, dst); err != nil {
				notes = append(notes, fmt.Sprintf("copy dir failed: %s: %v", clean, err))
				continue
			}
			exported = append(exported, dst)
			continue
		}
		if err := copyFileBestEffort(src, dst); err != nil {
			notes = append(notes, fmt.Sprintf("copy failed: %s: %v", clean, err))
			continue
		}
		exported = append(exported, dst)
	}

	sort.Strings(exported)
	return exported, notes
}

func copyDir(src, dst string) error {
	src = strings.TrimSpace(src)
	dst = strings.TrimSpace(dst)
	if src == "" || dst == "" {
		return errors.New("src and dst are required")
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		return copyFileBestEffort(path, target)
	})
}

func writeLedgerAndFindings(ledgerPath, findingsPath, ledgerSection, findingsSection string, inputs nodeInputsFile, diff gitutil.DiffArtifacts, exported []string, exportNotes []string) {
	_ = os.MkdirAll(filepath.Dir(ledgerPath), 0o700)
	_ = os.MkdirAll(filepath.Dir(findingsPath), 0o700)

	if strings.TrimSpace(ledgerSection) == "" {
		ledgerSection = "## 流水账\n- （无）"
	}
	_ = os.WriteFile(ledgerPath, []byte("# LEDGER\n\n"+strings.TrimSpace(ledgerSection)+"\n"), 0o600)

	var b strings.Builder
	b.WriteString("# FINDINGS\n\n")
	if strings.TrimSpace(findingsSection) != "" {
		b.WriteString(strings.TrimSpace(findingsSection))
		b.WriteString("\n\n")
	} else {
		b.WriteString("## Findings\n- （无）\n\n")
	}

	if len(inputs.Upstreams) > 0 {
		b.WriteString("## Inputs (paths only)\n")
		for _, up := range inputs.Upstreams {
			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(up.NodeID))
			b.WriteString("\n")
			if strings.TrimSpace(up.ManifestPath) != "" {
				b.WriteString("  - manifest: ")
				b.WriteString(strings.TrimSpace(up.ManifestPath))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("## Deliverables\n")
	if len(exported) == 0 {
		b.WriteString("- （无）\n")
	} else {
		for _, p := range exported {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(p)
			b.WriteString("\n")
		}
	}

	if strings.TrimSpace(diff.ChangedFilesPath) != "" {
		b.WriteString("\n## Diff evidence\n")
		b.WriteString("- changed_files: ")
		b.WriteString(strings.TrimSpace(diff.ChangedFilesPath))
		b.WriteString("\n")
		if strings.TrimSpace(diff.DiffPatchPath) != "" {
			b.WriteString("- diff_patch: ")
			b.WriteString(strings.TrimSpace(diff.DiffPatchPath))
			b.WriteString("\n")
		}
		if strings.TrimSpace(diff.Reason) != "" {
			b.WriteString("- note: ")
			b.WriteString(strings.TrimSpace(diff.Reason))
			b.WriteString("\n")
		}
	}

	if len(exportNotes) > 0 {
		b.WriteString("\n## Export notes\n")
		for _, note := range exportNotes {
			note = strings.TrimSpace(note)
			if note == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(note)
			b.WriteString("\n")
		}
	}

	_ = os.WriteFile(findingsPath, []byte(b.String()), 0o600)
}

func buildManifest(inputsPath, ledgerPath, findingsPath, tracePath, manifestPath string, diff gitutil.DiffArtifacts, deliverables []string) workflow.ArtifactManifest {
	arts := make([]workflow.Artifact, 0, 8+len(deliverables))

	add := func(kind, path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		arts = append(arts, workflow.Artifact{Kind: strings.TrimSpace(kind), Path: path})
	}

	add("inputs", inputsPath)
	add("ledger", ledgerPath)
	add("findings", findingsPath)
	add("trace", tracePath)
	add("manifest", manifestPath)
	add("changed_files", diff.ChangedFilesPath)
	add("diff_patch", diff.DiffPatchPath)

	for _, p := range deliverables {
		add("deliverable", p)
	}

	return workflow.ArtifactManifest{Artifacts: arts}
}

func writeChangedFilesOnly(nodeRoot, workspaceRoot string, changedFiles []string) gitutil.DiffArtifacts {
	nodeRoot = strings.TrimSpace(nodeRoot)
	workspaceRoot = strings.TrimSpace(workspaceRoot)

	createdAt := time.Now().UTC().Format(time.RFC3339)
	changedFilesPath := filepath.Join(nodeRoot, "changed_files.txt")
	_ = os.MkdirAll(filepath.Dir(changedFilesPath), 0o700)

	var b strings.Builder
	b.WriteString("# changed_files\n")
	b.WriteString("# generated_at: " + createdAt + "\n")
	b.WriteString("# workspace_root: " + workspaceRoot + "\n")
	b.WriteString("# git_workspace: false\n")
	b.WriteString("# note: not a git workspace (best-effort)\n\n")
	if len(changedFiles) == 0 {
		b.WriteString("- (none)\n")
	} else {
		for _, p := range changedFiles {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			b.WriteString(p)
			b.WriteString("\n")
		}
	}
	_ = os.WriteFile(changedFilesPath, []byte(b.String()), 0o600)

	return gitutil.DiffArtifacts{
		DiffPatchPath:    "",
		ChangedFilesPath: changedFilesPath,
		Reason:           "not a git workspace",
		IsGitWorkspace:   false,
	}
}
