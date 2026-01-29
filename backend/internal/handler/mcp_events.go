package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

// StreamMCPTaskEvents provides a best-effort event stream (SSE) for task attempt status changes.
// This is an MCP-adjacent transport to support notifications in external clients.
func StreamMCPTaskEvents(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Tasks == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task store not initialized"})
		return
	}

	principal := strings.TrimSpace(middleware.GetUserID(c))
	if principal == "" {
		principal = "local"
	}

	snap, err := rt.ResolveToolPolicySnapshot(c.Request.Context(), principal)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	if decision := permissions.Evaluate(snap.Policy, "mcp.events.tasks"); !decision.Allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	workspace := strings.TrimSpace(c.Query("workspace"))

	interval := 1 * time.Second
	if raw := strings.TrimSpace(c.Query("interval_ms")); raw != "" {
		if ms, err := time.ParseDuration(raw + "ms"); err == nil {
			if ms < 250*time.Millisecond {
				ms = 250 * time.Millisecond
			}
			if ms > 5*time.Second {
				ms = 5 * time.Second
			}
			interval = ms
		}
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeaderNow()

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}

	send := func(event string, data string) {
		_, _ = fmt.Fprintf(c.Writer, "event: %s\n", event)
		_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	}

	prev := map[string]string{}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	send("hello", fmt.Sprintf(`{"ts":%q,"principal_id":%q}`, now, principal))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			tasks, err := rt.Tasks.ListTasks(principal, workspace)
			if err != nil {
				continue
			}

			for _, t := range tasks {
				attemptID := ""
				status := ""
				if len(t.Attempts) > 0 {
					a := t.Attempts[len(t.Attempts)-1]
					attemptID = a.ID
					status = string(a.Status)
				}
				key := attemptID + ":" + status
				if prev[t.ID] == "" {
					prev[t.ID] = key
					continue
				}
				if prev[t.ID] == key {
					continue
				}
				prev[t.ID] = key
				send("attempt.status", fmt.Sprintf(
					`{"ts":%q,"task_id":%q,"attempt_id":%q,"status":%q,"title":%q,"workspace":%q}`,
					time.Now().UTC().Format(time.RFC3339Nano),
					t.ID,
					attemptID,
					status,
					t.Title,
					t.Workspace,
				))
			}
		}
	}
}
