package handler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/skill"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
	"github.com/liu_y/oneAgent/backend/internal/tool"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

type mcpJSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpJSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type mcpJSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpJSONRPCError `json:"error,omitempty"`
}

func HandleMCP(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	principal := strings.TrimSpace(middleware.GetUserID(c))
	if principal == "" {
		principal = "local"
	}

	var req mcpJSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	method := strings.TrimSpace(req.Method)
	if method == "" {
		writeMCPError(c, http.StatusOK, req.ID, -32600, "invalid request", "missing method")
		return
	}

	toolID, toolName, argsRaw := mcpPolicyToolID(method, req.Params)
	trace := mcpTraceEntry{
		TS:          time.Now().UTC().Format(time.RFC3339Nano),
		PrincipalID: principal,
		Method:      method,
		ToolID:      toolID,
		ToolName:    toolName,
	}
	if len(argsRaw) > 0 {
		trace.ArgsHash = toolArgsHash(argsRaw)
		trace.RequestID = extractRequestID(argsRaw)
	}
	defer func() { appendMCPTrace(rt, trace) }()

	snap, err := rt.ResolveToolPolicySnapshot(c.Request.Context(), principal)
	if err != nil {
		trace.OK = false
		trace.Error = err.Error()
		writeMCPError(c, http.StatusInternalServerError, req.ID, -32603, "internal error", err.Error())
		return
	}

	if toolID != "" {
		decision := permissions.Evaluate(snap.Policy, toolID)
		if isMCPWriteToolID(toolID) {
			// For mutating MCP tools, require an explicit allow rule (opt-in).
			if !decision.Allowed || decision.Reason != "allowed_by_rule" {
				trace.OK = false
				trace.Error = "forbidden"
				writeMCPError(c, http.StatusForbidden, req.ID, -32000, "forbidden", map[string]any{
					"code":    "forbidden",
					"tool_id": toolID,
					"reason":  decision.Reason,
					"rule_id": decision.RuleID,
					"request_id": func() string {
						if trace.RequestID != "" {
							return trace.RequestID
						}
						return ""
					}(),
				})
				return
			}
		} else if !decision.Allowed {
			trace.OK = false
			trace.Error = "forbidden"
			writeMCPError(c, http.StatusForbidden, req.ID, -32000, "forbidden", map[string]any{
				"code":    "forbidden",
				"tool_id": toolID,
				"reason":  decision.Reason,
				"rule_id": decision.RuleID,
				"request_id": func() string {
					if trace.RequestID != "" {
						return trace.RequestID
					}
					return ""
				}(),
			})
			return
		}
	}

	switch method {
	case "ping":
		trace.OK = true
		writeMCPResult(c, http.StatusOK, req.ID, map[string]any{})
		return
	case "initialize":
		trace.OK = true
		writeMCPResult(c, http.StatusOK, req.ID, map[string]any{
			"serverInfo": map[string]any{
				"name": "oneagent",
			},
			"capabilities": map[string]any{
				"tools":     true,
				"resources": true,
				"events":    true,
			},
		})
		return
	case "tools/list":
		trace.OK = true
		writeMCPResult(c, http.StatusOK, req.ID, map[string]any{"tools": mcpToolsList()})
		return
	case "tools/call":
		result, outcome, callErr := mcpCallTool(c.Request.Context(), rt, snap, principal, toolID, req.Params)
		if callErr != nil {
			trace.OK = false
			trace.Error = callErr.Error()

			var approvalRequired *tool.ApprovalRequiredError
			var approvalDenied *tool.ApprovalDeniedError
			var invalidArgs *tool.InvalidArgumentsError
			if errors.As(callErr, &approvalRequired) {
				trace.Error = "approval_required"
				trace.ApprovalID = strings.TrimSpace(approvalRequired.ApprovalID)
				writeMCPError(c, http.StatusOK, req.ID, -32001, "approval required", map[string]any{
					"code":              "approval_required",
					"approval_required": true,
					"approval_id":       approvalRequired.ApprovalID,
					"tool_id":           approvalRequired.ToolID,
					"scope_id":          approvalRequired.ScopeID,
					"request_id":        outcome.RequestID,
					"args_hash":         outcome.ArgsHash,
				})
				return
			} else if errors.As(callErr, &approvalDenied) {
				trace.Error = "approval_denied"
				trace.ApprovalID = strings.TrimSpace(approvalDenied.ApprovalID)
				writeMCPError(c, http.StatusOK, req.ID, -32002, "approval denied", map[string]any{
					"code":            "approval_denied",
					"approval_denied": true,
					"approval_id":     approvalDenied.ApprovalID,
					"tool_id":         approvalDenied.ToolID,
					"scope_id":        approvalDenied.ScopeID,
					"reason":          approvalDenied.Reason,
					"request_id":      outcome.RequestID,
					"args_hash":       outcome.ArgsHash,
				})
				return
			} else if errors.As(callErr, &invalidArgs) {
				trace.Error = "invalid_arguments"
				writeMCPError(c, http.StatusOK, req.ID, -32602, "invalid params", map[string]any{
					"code":           "invalid_arguments",
					"message":        strings.TrimSpace(callErr.Error()),
					"missing_fields": invalidArgs.MissingFields,
					"request_id":     outcome.RequestID,
					"args_hash":      outcome.ArgsHash,
				})
				return
			}

			writeMCPError(c, http.StatusOK, req.ID, -32000, "tool call failed", map[string]any{
				"code":       "tool_call_failed",
				"message":    strings.TrimSpace(callErr.Error()),
				"request_id": outcome.RequestID,
				"args_hash":  outcome.ArgsHash,
			})
			return
		}
		trace.OK = true
		if outcome.RequestID != "" {
			trace.RequestID = outcome.RequestID
		}
		if outcome.ArgsHash != "" {
			trace.ArgsHash = outcome.ArgsHash
		}
		trace.TaskID = outcome.TaskID
		trace.AttemptID = outcome.AttemptID
		writeMCPResult(c, http.StatusOK, req.ID, result)
		return
	case "resources/list":
		trace.OK = true
		writeMCPResult(c, http.StatusOK, req.ID, map[string]any{"resources": []any{}})
		return
	default:
		trace.OK = false
		trace.Error = "method_not_found"
		writeMCPError(c, http.StatusOK, req.ID, -32601, "method not found", method)
		return
	}
}

func mcpPolicyToolID(method string, params json.RawMessage) (toolID string, toolName string, args json.RawMessage) {
	method = strings.TrimSpace(method)
	if method == "" {
		return "", "", nil
	}

	if method == "tools/call" {
		var call struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments,omitempty"`
		}
		if len(params) > 0 && json.Unmarshal(params, &call) == nil {
			name := strings.TrimSpace(call.Name)
			if name != "" {
				return "mcp.tool." + name, name, call.Arguments
			}
		}
	}

	method = strings.ReplaceAll(method, "/", ".")
	return "mcp." + method, "", nil
}

func isMCPWriteToolID(toolID string) bool {
	switch strings.TrimSpace(toolID) {
	case "mcp.tool.tasks.create", "mcp.tool.tasks.cancel", "mcp.tool.tasks.resume":
		return true
	default:
		return false
	}
}

func writeMCPResult(c *gin.Context, status int, id any, result any) {
	c.JSON(status, mcpJSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func writeMCPError(c *gin.Context, status int, id any, code int, message string, data any) {
	c.JSON(status, mcpJSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcpJSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	})
}

type mcpTraceEntry struct {
	TS          string `json:"ts"`
	PrincipalID string `json:"principal_id"`
	Method      string `json:"method"`
	ToolID      string `json:"tool_id,omitempty"`
	ToolName    string `json:"tool_name,omitempty"`
	RequestID   string `json:"request_id,omitempty"`
	ArgsHash    string `json:"args_hash,omitempty"`
	TaskID      string `json:"task_id,omitempty"`
	AttemptID   string `json:"attempt_id,omitempty"`
	ApprovalID  string `json:"approval_id,omitempty"`
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
}

func appendMCPTrace(rt *runtime.Runtime, entry mcpTraceEntry) {
	if rt == nil || rt.Layout == nil {
		return
	}
	dir := strings.TrimSpace(rt.Layout.TraceLogsDir)
	if dir == "" {
		return
	}
	path := filepath.Join(dir, "mcp_calls.jsonl")

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')

	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
}

func mcpToolsList() []any {
	return []any{
		map[string]any{
			"name":        "tasks.list",
			"description": "List tasks (read-only).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"workspace": map[string]any{"type": "string"},
				},
			},
		},
		map[string]any{
			"name":        "tasks.get",
			"description": "Get a single task by id (read-only).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string"},
				},
				"required": []string{"task_id"},
			},
		},
		map[string]any{
			"name":        "receipts.list",
			"description": "List receipts (read-only).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"workspace": map[string]any{"type": "string"},
					"status":    map[string]any{"type": "string"},
					"q":         map[string]any{"type": "string"},
					"limit":     map[string]any{"type": "integer"},
				},
			},
		},
		map[string]any{
			"name":        "receipts.get",
			"description": "Get a single receipt by id (read-only).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"receipt_id": map[string]any{"type": "string"},
				},
				"required": []string{"receipt_id"},
			},
		},
		map[string]any{
			"name":        "skills.list",
			"description": "List skills (read-only).",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		map[string]any{
			"name":        "tasks.create",
			"description": "Create a task (write; requires explicit allow rule).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"request_id": map[string]any{"type": "string"},
					"workspace": map[string]any{"type": "string"},
					"title":     map[string]any{"type": "string"},
					"prompt":    map[string]any{"type": "string"},
					"model_id":  map[string]any{"type": "string"},
				},
				"required": []string{"request_id", "workspace", "prompt"},
			},
		},
		map[string]any{
			"name":        "tasks.cancel",
			"description": "Cancel the selected task (write; requires explicit allow rule).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"request_id": map[string]any{"type": "string"},
					"task_id": map[string]any{"type": "string"},
				},
				"required": []string{"request_id", "task_id"},
			},
		},
		map[string]any{
			"name":        "tasks.resume",
			"description": "Resume the selected task (write; requires explicit allow rule).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"request_id":   map[string]any{"type": "string"},
					"task_id":     map[string]any{"type": "string"},
					"review_notes": map[string]any{"type": "string"},
				},
				"required": []string{"request_id", "task_id"},
			},
		},
	}
}

type mcpToolOutcome struct {
	RequestID string
	ArgsHash  string
	TaskID    string
	AttemptID string
}

func mcpCallTool(ctx context.Context, rt *runtime.Runtime, snap permissions.Snapshot, principal string, toolID string, params json.RawMessage) (any, mcpToolOutcome, error) {
	if rt == nil {
		return nil, mcpToolOutcome{}, errors.New("runtime not initialized")
	}

	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, mcpToolOutcome{}, errors.New("invalid params")
	}
	name := strings.TrimSpace(call.Name)
	if name == "" {
		return nil, mcpToolOutcome{}, errors.New("missing tool name")
	}
	argsHash := toolArgsHash(call.Arguments)
	outcome := mcpToolOutcome{ArgsHash: argsHash, RequestID: strings.TrimSpace(extractRequestID(call.Arguments))}
	if strings.TrimSpace(toolID) == "" {
		toolID = "mcp.tool." + name
	}

	requireApproval := func(requestID string) error {
		requestID = strings.TrimSpace(requestID)
		if requestID == "" {
			return &tool.InvalidArgumentsError{MissingFields: []string{"request_id"}}
		}
		toolCtx := tool.ContextWithPolicySnapshot(ctx, snap)
		toolCtx = tool.ContextWithSettingsDB(toolCtx, rt.Settings)
		toolCtx = tool.ContextWithSessionID(toolCtx, "mcp:"+requestID)
		return tool.RequireApprovalIfNeeded(toolCtx, toolID, call.Arguments)
	}

	switch name {
	case "tasks.list":
		if rt.Tasks == nil {
			return nil, mcpToolOutcome{}, errors.New("task store not initialized")
		}
		var args struct {
			Workspace string `json:"workspace"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		tasks, err := rt.Tasks.ListTasks(principal, args.Workspace)
		if err != nil {
			return nil, mcpToolOutcome{}, err
		}
		res, err := mcpToolResult(tasks)
		return res, outcome, err
	case "tasks.get":
		if rt.Tasks == nil {
			return nil, mcpToolOutcome{}, errors.New("task store not initialized")
		}
		var args struct {
			TaskID string `json:"task_id"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, mcpToolOutcome{}, errors.New("invalid arguments")
		}
		taskID := strings.TrimSpace(args.TaskID)
		if taskID == "" {
			return nil, mcpToolOutcome{}, errors.New("task_id is required")
		}
		t, err := rt.Tasks.GetTask(taskID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, mcpToolOutcome{}, errors.New("task not found")
			}
			return nil, mcpToolOutcome{}, err
		}
		if strings.TrimSpace(t.UserID) != strings.TrimSpace(principal) {
			return nil, mcpToolOutcome{}, errors.New("task not found")
		}
		res, err := mcpToolResult(t)
		return res, outcome, err
	case "receipts.list":
		if rt.WorkLedger == nil {
			return nil, mcpToolOutcome{}, errors.New("work ledger not initialized")
		}
		var args struct {
			Workspace string `json:"workspace"`
			Status    string `json:"status"`
			Q         string `json:"q"`
			Limit     int    `json:"limit"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		list, err := rt.WorkLedger.ListReceipts(workledger.ListReceiptsQuery{
			PrincipalID: strings.TrimSpace(principal),
			Workspace:   strings.TrimSpace(args.Workspace),
			Status:      workledger.ReceiptStatus(strings.TrimSpace(args.Status)),
			Q:           strings.TrimSpace(args.Q),
			Limit:       args.Limit,
		})
		if err != nil {
			return nil, mcpToolOutcome{}, err
		}
		res, err := mcpToolResult(list)
		return res, outcome, err
	case "receipts.get":
		if rt.WorkLedger == nil {
			return nil, mcpToolOutcome{}, errors.New("work ledger not initialized")
		}
		var args struct {
			ReceiptID string `json:"receipt_id"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, mcpToolOutcome{}, errors.New("invalid arguments")
		}
		id := strings.TrimSpace(args.ReceiptID)
		if id == "" {
			return nil, mcpToolOutcome{}, errors.New("receipt_id is required")
		}
		r, err := rt.WorkLedger.GetReceipt(id)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, mcpToolOutcome{}, errors.New("receipt not found")
			}
			return nil, mcpToolOutcome{}, err
		}
		if strings.TrimSpace(r.PrincipalID) != strings.TrimSpace(principal) {
			return nil, mcpToolOutcome{}, errors.New("receipt not found")
		}
		res, err := mcpToolResult(r)
		return res, outcome, err
	case "skills.list":
		cat, err := skill.Discover(ctx, skill.DiscoverOptions{})
		if err != nil {
			return nil, mcpToolOutcome{}, err
		}
		home := ""
		if rt.Config != nil {
			home = strings.TrimSpace(rt.Config.Home)
		}
		out := make([]any, 0, len(cat.Skills))
		for _, s := range cat.Skills {
			out = append(out, map[string]any{
				"skill_id":     s.ID,
				"name":         s.Name,
				"description":  s.Description,
				"source":       s.Source,
				"path":         s.Path,
				"archivable":   canArchiveSkill(home, s),
			})
		}
		res, err := mcpToolResult(out)
		return res, outcome, err
	case "tasks.create":
		if rt.Tasks == nil || rt.TaskRunner == nil {
			return nil, mcpToolOutcome{}, errors.New("task queue not initialized")
		}
		var args struct {
			RequestID string `json:"request_id"`
			Workspace string `json:"workspace"`
			Title     string `json:"title"`
			Prompt    string `json:"prompt"`
			ModelID   string `json:"model_id"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, mcpToolOutcome{}, errors.New("invalid arguments")
		}
		if err := requireApproval(args.RequestID); err != nil {
			return nil, outcome, err
		}
		ws := strings.TrimSpace(args.Workspace)
		prompt := strings.TrimSpace(args.Prompt)
		if ws == "" || prompt == "" {
			return nil, mcpToolOutcome{}, errors.New("workspace and prompt are required")
		}
		title := strings.TrimSpace(args.Title)
		if title == "" {
			title = deriveTaskTitle(prompt)
		}
		created, err := rt.Tasks.CreateTask(principal, ws, title, prompt, args.ModelID, taskqueue.Limits{})
		if err != nil {
			return nil, mcpToolOutcome{}, err
		}
		if err := rt.TaskRunner.Enqueue(created.ID); err != nil {
			return nil, mcpToolOutcome{}, err
		}
		if latest := created.LatestAttempt(); latest != nil {
			outcome.AttemptID = latest.ID
		}
		outcome.TaskID = created.ID
		outcome.RequestID = strings.TrimSpace(args.RequestID)

		_ = rt.Tasks.AppendEvent(taskqueue.Event{
			TaskID:    created.ID,
			AttemptID: outcome.AttemptID,
			Type:      "mcp.action.tasks.create",
			Message:   "Task created via MCP",
			Data: map[string]any{
				"principal_id": strings.TrimSpace(principal),
				"request_id":   outcome.RequestID,
				"tool_id":      strings.TrimSpace(toolID),
				"args_hash":    outcome.ArgsHash,
			},
		})

		res, err := mcpToolResult(created)
		return res, outcome, err
	case "tasks.cancel":
		if rt.Tasks == nil || rt.TaskRunner == nil {
			return nil, mcpToolOutcome{}, errors.New("task queue not initialized")
		}
		var args struct {
			RequestID string `json:"request_id"`
			TaskID     string `json:"task_id"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, mcpToolOutcome{}, errors.New("invalid arguments")
		}
		if err := requireApproval(args.RequestID); err != nil {
			return nil, outcome, err
		}
		taskID := strings.TrimSpace(args.TaskID)
		if taskID == "" {
			return nil, mcpToolOutcome{}, errors.New("task_id is required")
		}
		task, err := rt.Tasks.GetTask(taskID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, mcpToolOutcome{}, errors.New("task not found")
			}
			return nil, mcpToolOutcome{}, err
		}
		if strings.TrimSpace(task.UserID) != strings.TrimSpace(principal) {
			return nil, mcpToolOutcome{}, errors.New("task not found")
		}
		updated, err := rt.TaskRunner.Cancel(taskID)
		if err != nil {
			return nil, mcpToolOutcome{}, err
		}
		if latest := updated.LatestAttempt(); latest != nil {
			outcome.AttemptID = latest.ID
		}
		outcome.TaskID = updated.ID
		outcome.RequestID = strings.TrimSpace(args.RequestID)

		_ = rt.Tasks.AppendEvent(taskqueue.Event{
			TaskID:    updated.ID,
			AttemptID: outcome.AttemptID,
			Type:      "mcp.action.tasks.cancel",
			Message:   "Task cancel requested via MCP",
			Data: map[string]any{
				"principal_id": strings.TrimSpace(principal),
				"request_id":   outcome.RequestID,
				"tool_id":      strings.TrimSpace(toolID),
				"args_hash":    outcome.ArgsHash,
			},
		})

		res, err := mcpToolResult(updated)
		return res, outcome, err
	case "tasks.resume":
		if rt.Tasks == nil || rt.TaskRunner == nil {
			return nil, mcpToolOutcome{}, errors.New("task queue not initialized")
		}
		var args struct {
			RequestID   string `json:"request_id"`
			TaskID      string `json:"task_id"`
			ReviewNotes string `json:"review_notes"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, mcpToolOutcome{}, errors.New("invalid arguments")
		}
		if err := requireApproval(args.RequestID); err != nil {
			return nil, outcome, err
		}
		taskID := strings.TrimSpace(args.TaskID)
		if taskID == "" {
			return nil, mcpToolOutcome{}, errors.New("task_id is required")
		}
		task, err := rt.Tasks.GetTask(taskID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, mcpToolOutcome{}, errors.New("task not found")
			}
			return nil, mcpToolOutcome{}, err
		}
		if strings.TrimSpace(task.UserID) != strings.TrimSpace(principal) {
			return nil, mcpToolOutcome{}, errors.New("task not found")
		}
		updated, err := rt.TaskRunner.ResumeWithSource(taskID, strings.TrimSpace(args.ReviewNotes), "mcp")
		if err != nil {
			return nil, mcpToolOutcome{}, err
		}
		if latest := updated.LatestAttempt(); latest != nil {
			outcome.AttemptID = latest.ID
		}
		outcome.TaskID = updated.ID
		outcome.RequestID = strings.TrimSpace(args.RequestID)

		_ = rt.Tasks.AppendEvent(taskqueue.Event{
			TaskID:    updated.ID,
			AttemptID: outcome.AttemptID,
			Type:      "mcp.action.tasks.resume",
			Message:   "Task resumed via MCP",
			Data: map[string]any{
				"principal_id": strings.TrimSpace(principal),
				"request_id":   outcome.RequestID,
				"tool_id":      strings.TrimSpace(toolID),
				"args_hash":    outcome.ArgsHash,
			},
		})

		res, err := mcpToolResult(updated)
		return res, outcome, err
	default:
		return nil, mcpToolOutcome{}, errors.New("unknown tool")
	}
}

func toolArgsHash(raw json.RawMessage) string {
	normalized := []byte(raw)
	if len(raw) > 0 {
		var v any
		if err := json.Unmarshal(raw, &v); err == nil {
			if b, err := json.Marshal(v); err == nil {
				normalized = b
			}
		}
	}
	sum := sha256.Sum256(normalized)
	return hex.EncodeToString(sum[:])
}

func extractRequestID(raw json.RawMessage) string {
	var v map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &v) != nil {
		return ""
	}
	if s, ok := v["request_id"].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func mcpToolResult(v any) (any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": string(data),
			},
		},
	}, nil
}
