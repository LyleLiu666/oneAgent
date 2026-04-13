package formalmemory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	agentsdk "codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/agentsdk.git"
	agentsdkbridge "codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git/bridge/agentsdk"
	"codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git/core"
	postgresstore "codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git/store/postgres"
)

type Config struct {
	PostgresDSN     string
	PreRecallPolicy string
}

type PreRecallRequest struct {
	RunID         string
	TurnID        string
	UserID        string
	SessionID     string
	WorkspaceRoot string
	AgentID       string
}

type PreRecallResult struct {
	TurnContext string
	Degraded    bool
}

type Service struct {
	catalog         *agentsdk.ToolCatalog
	bridge          *agentsdkbridge.Bridge
	preRecallPolicy agentsdkbridge.PreRecallPolicy
	closeFn         func()
}

type contextKey string

const hostContextKey contextKey = "formalmemory_host_context"

type ToolInvokeRequest struct {
	Name          string
	Arguments     json.RawMessage
	RunID         string
	TurnID        string
	UserID        string
	SessionID     string
	WorkspaceRoot string
	AgentID       string
	Protocol      string
	ToolCallID    string
}

type TurnEndRequest struct {
	RunID         string
	TurnID        string
	UserID        string
	SessionID     string
	WorkspaceRoot string
	AgentID       string
	Payload       json.RawMessage
	NotBefore     *time.Time
}

type TurnEndResult struct {
	AppliedAction string
	Enqueued      bool
	JobID         string
}

func Open(ctx context.Context, cfg Config) (*Service, error) {
	dsn := strings.TrimSpace(cfg.PostgresDSN)
	if dsn == "" {
		return nil, nil
	}

	store, err := postgresstore.Open(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open memorysdk postgres store: %w", err)
	}
	if err := store.ApplyMigrations(ctx); err != nil {
		store.Close()
		return nil, fmt.Errorf("apply memorysdk migrations: %w", err)
	}

	svc, err := NewServiceWithJobStore(store, store, cfg)
	if err != nil {
		store.Close()
		return nil, err
	}
	svc.closeFn = store.Close
	return svc, nil
}

func NewService(store core.MemoryStore, cfg Config) (*Service, error) {
	var jobStore core.JobStateStore
	if typed, ok := store.(core.JobStateStore); ok {
		jobStore = typed
	}
	return NewServiceWithJobStore(store, jobStore, cfg)
}

func NewServiceWithJobStore(store core.MemoryStore, jobStore core.JobStateStore, cfg Config) (*Service, error) {
	if store == nil {
		return nil, errors.New("memory store is required")
	}

	policy, err := normalizePreRecallPolicy(cfg.PreRecallPolicy)
	if err != nil {
		return nil, err
	}

	catalog := agentsdk.NewToolCatalog()
	bridge, err := agentsdkbridge.Install(agentsdkbridge.Config{
		Catalog:           catalog,
		MemoryStore:       store,
		JobStore:          jobStore,
		HostContextSource: hostContextFromContext,
		EventSink:         agentsdkbridge.BridgeEventSinkFunc(logBridgeEvent),
	})
	if err != nil {
		return nil, fmt.Errorf("install memorysdk bridge: %w", err)
	}

	return &Service{
		catalog:         catalog,
		bridge:          bridge,
		preRecallPolicy: policy,
	}, nil
}

func (s *Service) Close() error {
	if s == nil || s.closeFn == nil {
		return nil
	}
	s.closeFn()
	return nil
}

func (s *Service) PreRecallTurnContext(ctx context.Context, req PreRecallRequest) (PreRecallResult, error) {
	if s == nil || s.bridge == nil {
		return PreRecallResult{}, nil
	}

	hostCtx := buildHostContext(req, s.preRecallPolicy)
	ctx = context.WithValue(ctx, hostContextKey, hostCtx)

	result, degraded, err := s.bridge.PreRecall(ctx)
	if err != nil {
		return PreRecallResult{}, err
	}
	return PreRecallResult{
		TurnContext: formatPreRecallTurnContext(result),
		Degraded:    degraded,
	}, nil
}

func (s *Service) ExecuteTool(ctx context.Context, req ToolInvokeRequest) (any, error) {
	if s == nil || s.catalog == nil {
		return nil, errors.New("formal memory tools are not initialized")
	}

	hostCtx := buildHostContext(PreRecallRequest{
		RunID:         req.RunID,
		TurnID:        req.TurnID,
		UserID:        req.UserID,
		SessionID:     req.SessionID,
		WorkspaceRoot: req.WorkspaceRoot,
		AgentID:       req.AgentID,
	}, s.preRecallPolicy)
	ctx = context.WithValue(ctx, hostContextKey, hostCtx)

	protocol := strings.ToLower(strings.TrimSpace(req.Protocol))
	if protocol == "" {
		protocol = "json"
	}

	return s.catalog.Execute(ctx, agentsdk.ToolInvocation{
		ID:            strings.TrimSpace(req.ToolCallID),
		Name:          strings.TrimSpace(req.Name),
		ArgumentsJSON: req.Arguments,
		Protocol:      protocol,
	})
}

func (s *Service) EnqueueTurnEndExtractJob(ctx context.Context, req TurnEndRequest) (TurnEndResult, error) {
	if s == nil || s.bridge == nil {
		return TurnEndResult{}, errors.New("formal memory is not initialized")
	}

	hostCtx := buildHostContext(PreRecallRequest{
		RunID:         req.RunID,
		TurnID:        req.TurnID,
		UserID:        req.UserID,
		SessionID:     req.SessionID,
		WorkspaceRoot: req.WorkspaceRoot,
		AgentID:       req.AgentID,
	}, s.preRecallPolicy)
	ctx = context.WithValue(ctx, hostContextKey, hostCtx)

	result, err := s.bridge.TurnEnd(ctx, agentsdkbridge.TurnEndRequest{
		Action:    agentsdkbridge.TurnEndActionEnqueueExtractJob,
		Payload:   req.Payload,
		NotBefore: req.NotBefore,
	})
	if err != nil {
		return TurnEndResult{}, err
	}
	return TurnEndResult{
		AppliedAction: strings.TrimSpace(result.AppliedAction),
		Enqueued:      result.Enqueued,
		JobID:         strings.TrimSpace(result.JobID),
	}, nil
}

func normalizePreRecallPolicy(raw string) (agentsdkbridge.PreRecallPolicy, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return agentsdkbridge.PreRecallPolicyAuto, nil
	}

	switch agentsdkbridge.PreRecallPolicy(value) {
	case agentsdkbridge.PreRecallPolicyNone,
		agentsdkbridge.PreRecallPolicySessionOnly,
		agentsdkbridge.PreRecallPolicyAuto:
		return agentsdkbridge.PreRecallPolicy(value), nil
	default:
		return "", fmt.Errorf("invalid memorysdk pre-recall policy: %q", raw)
	}
}

func buildHostContext(req PreRecallRequest, policy agentsdkbridge.PreRecallPolicy) agentsdkbridge.HostMemoryContext {
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		userID = "local"
	}

	sessionID := strings.TrimSpace(req.SessionID)
	workspaceRoot := strings.TrimSpace(req.WorkspaceRoot)
	projectID := projectScopeIDFromWorkspaceRoot(workspaceRoot)
	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		agentID = "oneagent.chat.assistant"
	}

	scopes := map[core.ScopeKind]string{}
	order := make([]core.ScopeKind, 0, 3)
	allowlist := make([]core.ScopeKind, 0, 3)

	if sessionID != "" {
		scopes[core.ScopeKindThread] = sessionID
		order = append(order, core.ScopeKindThread)
		allowlist = append(allowlist, core.ScopeKindThread)
	}
	if projectID != "" {
		scopes[core.ScopeKindProject] = projectID
		order = append(order, core.ScopeKindProject)
		allowlist = append(allowlist, core.ScopeKindProject)
	}
	if userID != "" {
		scopes[core.ScopeKindUser] = userID
		order = append(order, core.ScopeKindUser)
		allowlist = append(allowlist, core.ScopeKindUser)
	}

	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		runID = "chat:" + sessionID
	}
	turnID := strings.TrimSpace(req.TurnID)
	if turnID == "" {
		turnID = "turn:prerecall"
	}

	return agentsdkbridge.HostMemoryContext{
		RunID:                  runID,
		TurnID:                 turnID,
		UserID:                 userID,
		ProjectID:              projectID,
		ThreadID:               sessionID,
		AgentID:                agentID,
		Scopes:                 scopes,
		RecallScopeOrder:       order,
		RememberScopeAllowlist: allowlist,
		PreRecallPolicy:        policy,
	}
}

func hostContextFromContext(ctx context.Context) (agentsdkbridge.HostMemoryContext, error) {
	if ctx == nil {
		return agentsdkbridge.HostMemoryContext{}, errors.New("host memory context missing")
	}
	hostCtx, ok := ctx.Value(hostContextKey).(agentsdkbridge.HostMemoryContext)
	if !ok {
		return agentsdkbridge.HostMemoryContext{}, errors.New("host memory context missing")
	}
	return hostCtx, nil
}

func formatPreRecallTurnContext(result core.RecallResult) string {
	if len(result.Items) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("【FormalMemory 预召回】\n")
	b.WriteString("以下内容来自外部正式记忆，仅供参考，必要时请自行核实。\n")
	for i, item := range result.Items {
		summary := strings.TrimSpace(item.Summary)
		if summary == "" {
			summary = "(empty summary)"
		}
		b.WriteString(fmt.Sprintf("%d. [%s/%s] %s\n", i+1, item.ScopeKind, item.Type, summary))
		if sourceRef := strings.TrimSpace(item.SourceRef); sourceRef != "" {
			b.WriteString(fmt.Sprintf("   source_ref: %s\n", sourceRef))
		}
	}
	return strings.TrimSpace(b.String())
}

func logBridgeEvent(_ context.Context, event agentsdkbridge.BridgeEvent) error {
	log.Printf("formalmemory bridge event=%s stage=%s reason=%s target_scope=%s:%s target_id=%s candidate_id=%s formal_id=%s",
		strings.TrimSpace(event.EventName),
		strings.TrimSpace(event.Stage),
		strings.TrimSpace(event.ReasonCode),
		strings.TrimSpace(event.TargetScopeKind),
		bridgeEventScopeIDForLog(event.TargetScopeKind, event.TargetScopeID),
		strings.TrimSpace(event.TargetID),
		strings.TrimSpace(event.CandidateID),
		strings.TrimSpace(event.FormalID),
	)
	return nil
}

func bridgeEventScopeIDForLog(scopeKind, scopeID string) string {
	kind := strings.TrimSpace(scopeKind)
	id := strings.TrimSpace(scopeID)
	if kind == string(core.ScopeKindProject) && looksLikeAbsoluteHostPath(id) {
		return "[redacted-host-path]"
	}
	return id
}

func looksLikeAbsoluteHostPath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\\`) {
		return true
	}
	if len(value) >= 3 {
		ch := value[0]
		if ((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')) && value[1] == ':' && (value[2] == '\\' || value[2] == '/') {
			return true
		}
	}
	return false
}
