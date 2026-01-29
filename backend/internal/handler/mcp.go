package handler

import (
	"context"
	"encoding/json"
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

	toolID := mcpPolicyToolID(method, req.Params)
	trace := mcpTraceEntry{
		TS:          time.Now().UTC().Format(time.RFC3339Nano),
		PrincipalID: principal,
		Method:      method,
		ToolID:      toolID,
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
					"tool_id": toolID,
					"reason":  decision.Reason,
					"rule_id": decision.RuleID,
				})
				return
			}
		} else if !decision.Allowed {
			trace.OK = false
			trace.Error = "forbidden"
			writeMCPError(c, http.StatusForbidden, req.ID, -32000, "forbidden", map[string]any{
				"tool_id": toolID,
				"reason":  decision.Reason,
				"rule_id": decision.RuleID,
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
		result, callErr := mcpCallTool(c.Request.Context(), rt, principal, req.Params)
		if callErr != nil {
			trace.OK = false
			trace.Error = callErr.Error()
			writeMCPError(c, http.StatusOK, req.ID, -32000, "tool call failed", callErr.Error())
			return
		}
		trace.OK = true
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

func mcpPolicyToolID(method string, params json.RawMessage) string {
	method = strings.TrimSpace(method)
	if method == "" {
		return ""
	}

	if method == "tools/call" {
		var call struct {
			Name string `json:"name"`
		}
		if len(params) > 0 && json.Unmarshal(params, &call) == nil {
			name := strings.TrimSpace(call.Name)
			if name != "" {
				return "mcp.tool." + name
			}
		}
	}

	method = strings.ReplaceAll(method, "/", ".")
	return "mcp." + method
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
					"workspace": map[string]any{"type": "string"},
					"title":     map[string]any{"type": "string"},
					"prompt":    map[string]any{"type": "string"},
					"model_id":  map[string]any{"type": "string"},
				},
				"required": []string{"workspace", "prompt"},
			},
		},
		map[string]any{
			"name":        "tasks.cancel",
			"description": "Cancel the selected task (write; requires explicit allow rule).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string"},
				},
				"required": []string{"task_id"},
			},
		},
		map[string]any{
			"name":        "tasks.resume",
			"description": "Resume the selected task (write; requires explicit allow rule).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id":     map[string]any{"type": "string"},
					"review_notes": map[string]any{"type": "string"},
				},
				"required": []string{"task_id"},
			},
		},
	}
}

func mcpCallTool(ctx context.Context, rt *runtime.Runtime, principal string, params json.RawMessage) (any, error) {
	if rt == nil {
		return nil, errors.New("runtime not initialized")
	}

	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, errors.New("invalid params")
	}
	name := strings.TrimSpace(call.Name)
	if name == "" {
		return nil, errors.New("missing tool name")
	}

	switch name {
	case "tasks.list":
		if rt.Tasks == nil {
			return nil, errors.New("task store not initialized")
		}
		var args struct {
			Workspace string `json:"workspace"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		tasks, err := rt.Tasks.ListTasks(principal, args.Workspace)
		if err != nil {
			return nil, err
		}
		return mcpToolResult(tasks)
	case "tasks.get":
		if rt.Tasks == nil {
			return nil, errors.New("task store not initialized")
		}
		var args struct {
			TaskID string `json:"task_id"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, errors.New("invalid arguments")
		}
		taskID := strings.TrimSpace(args.TaskID)
		if taskID == "" {
			return nil, errors.New("task_id is required")
		}
		t, err := rt.Tasks.GetTask(taskID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, errors.New("task not found")
			}
			return nil, err
		}
		if strings.TrimSpace(t.UserID) != strings.TrimSpace(principal) {
			return nil, errors.New("task not found")
		}
		return mcpToolResult(t)
	case "receipts.list":
		if rt.WorkLedger == nil {
			return nil, errors.New("work ledger not initialized")
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
			return nil, err
		}
		return mcpToolResult(list)
	case "receipts.get":
		if rt.WorkLedger == nil {
			return nil, errors.New("work ledger not initialized")
		}
		var args struct {
			ReceiptID string `json:"receipt_id"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, errors.New("invalid arguments")
		}
		id := strings.TrimSpace(args.ReceiptID)
		if id == "" {
			return nil, errors.New("receipt_id is required")
		}
		r, err := rt.WorkLedger.GetReceipt(id)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, errors.New("receipt not found")
			}
			return nil, err
		}
		if strings.TrimSpace(r.PrincipalID) != strings.TrimSpace(principal) {
			return nil, errors.New("receipt not found")
		}
		return mcpToolResult(r)
	case "skills.list":
		cat, err := skill.Discover(ctx, skill.DiscoverOptions{})
		if err != nil {
			return nil, err
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
		return mcpToolResult(out)
	case "tasks.create":
		if rt.Tasks == nil || rt.TaskRunner == nil {
			return nil, errors.New("task queue not initialized")
		}
		var args struct {
			Workspace string `json:"workspace"`
			Title     string `json:"title"`
			Prompt    string `json:"prompt"`
			ModelID   string `json:"model_id"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, errors.New("invalid arguments")
		}
		ws := strings.TrimSpace(args.Workspace)
		prompt := strings.TrimSpace(args.Prompt)
		if ws == "" || prompt == "" {
			return nil, errors.New("workspace and prompt are required")
		}
		title := strings.TrimSpace(args.Title)
		if title == "" {
			title = deriveTaskTitle(prompt)
		}
		created, err := rt.Tasks.CreateTask(principal, ws, title, prompt, args.ModelID, taskqueue.Limits{})
		if err != nil {
			return nil, err
		}
		if err := rt.TaskRunner.Enqueue(created.ID); err != nil {
			return nil, err
		}
		return mcpToolResult(created)
	case "tasks.cancel":
		if rt.Tasks == nil || rt.TaskRunner == nil {
			return nil, errors.New("task queue not initialized")
		}
		var args struct {
			TaskID string `json:"task_id"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, errors.New("invalid arguments")
		}
		taskID := strings.TrimSpace(args.TaskID)
		if taskID == "" {
			return nil, errors.New("task_id is required")
		}
		task, err := rt.Tasks.GetTask(taskID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, errors.New("task not found")
			}
			return nil, err
		}
		if strings.TrimSpace(task.UserID) != strings.TrimSpace(principal) {
			return nil, errors.New("task not found")
		}
		updated, err := rt.TaskRunner.Cancel(taskID)
		if err != nil {
			return nil, err
		}
		return mcpToolResult(updated)
	case "tasks.resume":
		if rt.Tasks == nil || rt.TaskRunner == nil {
			return nil, errors.New("task queue not initialized")
		}
		var args struct {
			TaskID      string `json:"task_id"`
			ReviewNotes string `json:"review_notes"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, errors.New("invalid arguments")
		}
		taskID := strings.TrimSpace(args.TaskID)
		if taskID == "" {
			return nil, errors.New("task_id is required")
		}
		task, err := rt.Tasks.GetTask(taskID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, errors.New("task not found")
			}
			return nil, err
		}
		if strings.TrimSpace(task.UserID) != strings.TrimSpace(principal) {
			return nil, errors.New("task not found")
		}
		updated, err := rt.TaskRunner.Resume(taskID, strings.TrimSpace(args.ReviewNotes))
		if err != nil {
			return nil, err
		}
		return mcpToolResult(updated)
	default:
		return nil, errors.New("unknown tool")
	}
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
