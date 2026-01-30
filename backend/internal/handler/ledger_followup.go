package handler

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

type createLedgerFollowUpRequest struct {
	ReceiptIDs  []string         `json:"receipt_ids"`
	Instruction string           `json:"instruction,omitempty"`
	ModelID     string           `json:"model_id,omitempty"`
	Limits      taskqueue.Limits `json:"limits,omitempty"`
}

func CreateLedgerFollowUpTask(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.WorkLedger == nil || rt.Tasks == nil || rt.TaskRunner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	principal := strings.TrimSpace(middleware.GetUserID(c))
	if principal == "" {
		principal = "local"
	}

	var req createLedgerFollowUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ids := make([]string, 0, len(req.ReceiptIDs))
	for _, id := range req.ReceiptIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "receipt_ids is required"})
		return
	}

	type picked struct {
		id           string
		workspace    string
		status       string
		summary      string
		findingsPath string
		tracePath    string
	}

	workspace := ""
	items := make([]picked, 0, len(ids))
	for _, id := range ids {
		r, err := rt.WorkLedger.GetReceipt(id)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "receipt not found", "receipt_id": id})
				return
			}
			RespondError(c, http.StatusInternalServerError, err)
			return
		}
		if strings.TrimSpace(r.PrincipalID) != principal {
			c.JSON(http.StatusNotFound, gin.H{"error": "receipt not found", "receipt_id": id})
			return
		}

		ws := strings.TrimSpace(r.WorkspaceRoot)
		if ws == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "receipt has no workspace_root", "receipt_id": id})
			return
		}
		if workspace == "" {
			workspace = ws
		} else if workspace != ws {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":            "receipts must belong to the same workspace",
				"workspace_root_1": workspace,
				"workspace_root_2": ws,
				"receipt_id":       id,
			})
			return
		}

		items = append(items, picked{
			id:           r.ReceiptID,
			workspace:    ws,
			status:       string(r.Status),
			summary:      strings.TrimSpace(r.Summary),
			findingsPath: strings.TrimSpace(r.Artifacts.FindingsPath),
			tracePath:    strings.TrimSpace(r.Artifacts.TraceLogPath),
		})
	}

	var b strings.Builder
	b.WriteString("你正在基于一组 Work Ledger receipts 进行批处理 follow-up。\n\n")
	if strings.TrimSpace(req.Instruction) != "" {
		b.WriteString("用户补充指令：\n")
		b.WriteString(strings.TrimSpace(req.Instruction))
		b.WriteString("\n\n")
	}
	b.WriteString("必须先读证据，再决定下一步：\n")
	b.WriteString("- 优先阅读 findings_path\n")
	b.WriteString("- 需要时再读 trace_log_path\n\n")
	b.WriteString("Receipts（同一 workspace）：\n")
	for _, it := range items {
		b.WriteString("- receipt_id=")
		b.WriteString(it.id)
		b.WriteString(" status=")
		b.WriteString(it.status)
		if it.summary != "" {
			b.WriteString(" summary=")
			b.WriteString(strings.ReplaceAll(strings.Join(strings.Fields(it.summary), " "), "\n", " "))
		}
		b.WriteString("\n")
		if it.findingsPath != "" {
			b.WriteString("  - findings_path: ")
			b.WriteString(it.findingsPath)
			b.WriteString("\n")
		}
		if it.tracePath != "" {
			b.WriteString("  - trace_log_path: ")
			b.WriteString(it.tracePath)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n交付物要求：\n- 给出一个可执行的下一步 plan（可分批）\n- 如果需要改代码：小步提交、跑测试、保留证据\n")

	title := "Follow-up (" + strings.TrimSpace(workspace) + ")"
	task, err := rt.Tasks.CreateTask(principal, workspace, title, b.String(), req.ModelID, taskqueue.ResolveLimits(req.Limits))
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	if err := rt.TaskRunner.Enqueue(task.ID); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, task)
}

