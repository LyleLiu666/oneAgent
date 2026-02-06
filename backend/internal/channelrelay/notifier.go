package channelrelay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

type TaskLink struct {
	Provider    string `json:"provider,omitempty"`
	PrincipalID string `json:"principal_id,omitempty"`
	ChannelID   string `json:"channel_id,omitempty"`
	ThreadID    string `json:"thread_id,omitempty"`
	MessageID   string `json:"message_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
}

func FindLatestTaskLink(events []taskqueue.Event) (TaskLink, bool) {
	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]
		if strings.TrimSpace(ev.Type) != TaskLinkEventType {
			continue
		}
		d := ev.Data
		if d == nil {
			continue
		}
		link := TaskLink{
			Provider:    strings.TrimSpace(asString(d["provider"])),
			PrincipalID: strings.TrimSpace(asString(d["principal_id"])),
			ChannelID:   strings.TrimSpace(asString(d["channel_id"])),
			ThreadID:    strings.TrimSpace(asString(d["thread_id"])),
			MessageID:   strings.TrimSpace(asString(d["message_id"])),
			SessionID:   strings.TrimSpace(asString(d["session_id"])),
		}
		if link.Provider == "" {
			link.Provider = ProviderWebhookV1
		}
		if link.ChannelID == "" || link.ThreadID == "" {
			continue
		}
		return link, true
	}
	return TaskLink{}, false
}

type Notifier struct {
	OutboundURL string
	TraceDir    string
	Client      *http.Client
}

func (n *Notifier) NotifyTaskTerminal(ctx context.Context, link TaskLink, task taskqueue.Task, attempt taskqueue.Attempt) error {
	if n == nil {
		return errors.New("notifier is nil")
	}
	if strings.TrimSpace(link.ChannelID) == "" || strings.TrimSpace(link.ThreadID) == "" {
		return errors.New("channel_id and thread_id are required")
	}
	outboundURL := strings.TrimSpace(n.OutboundURL)
	if outboundURL == "" {
		_ = AppendTrace(n.TraceDir, TraceEntry{
			Direction:  "outbound",
			Provider:   link.Provider,
			Principal:  link.PrincipalID,
			ChannelID:  link.ChannelID,
			ThreadID:   link.ThreadID,
			MessageID:  link.MessageID,
			SessionID:  link.SessionID,
			TaskID:     task.ID,
			AttemptID:  attempt.ID,
			TaskStatus: string(attempt.Status),
			OK:         false,
			Error:      "outbound_url_not_configured",
		})
		return errors.New("channel relay outbound url is not configured")
	}

	client := n.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	text := buildTerminalNotificationText(task, attempt)
	payload := map[string]any{
		"provider":     strings.TrimSpace(link.Provider),
		"principal_id": strings.TrimSpace(link.PrincipalID),
		"channel_id":   strings.TrimSpace(link.ChannelID),
		"thread_id":    strings.TrimSpace(link.ThreadID),
		"task_id":      strings.TrimSpace(task.ID),
		"attempt_id":   strings.TrimSpace(attempt.ID),
		"status":       strings.TrimSpace(string(attempt.Status)),
		"title":        strings.TrimSpace(task.Title),
		"summary":      strings.TrimSpace(attempt.Summary),
		"artifacts": map[string]any{
			"findings_path":        strings.TrimSpace(attempt.FindingsPath),
			"trace_log_path":       strings.TrimSpace(attempt.TraceLogPath),
			"test_report_path":     strings.TrimSpace(attempt.TestReportPath),
			"diff_patch_path":      strings.TrimSpace(attempt.DiffPatchPath),
			"changed_files_path":   strings.TrimSpace(attempt.ChangedFilesPath),
			"review_comments_path": strings.TrimSpace(attempt.ReviewCommentsPath),
		},
		"text": text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var lastErr error
	for attemptNo := 1; attemptNo <= 3; attemptNo++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, outboundURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err == nil && resp != nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				_ = AppendTrace(n.TraceDir, TraceEntry{
					Direction:  "outbound",
					Provider:   link.Provider,
					Principal:  link.PrincipalID,
					ChannelID:  link.ChannelID,
					ThreadID:   link.ThreadID,
					MessageID:  link.MessageID,
					SessionID:  link.SessionID,
					TaskID:     task.ID,
					AttemptID:  attempt.ID,
					TaskStatus: string(attempt.Status),
					OK:         true,
				})
				return nil
			}
			err = fmt.Errorf("outbound status=%d", resp.StatusCode)
		}

		lastErr = err
		_ = AppendTrace(n.TraceDir, TraceEntry{
			Direction:  "outbound",
			Provider:   link.Provider,
			Principal:  link.PrincipalID,
			ChannelID:  link.ChannelID,
			ThreadID:   link.ThreadID,
			MessageID:  link.MessageID,
			SessionID:  link.SessionID,
			TaskID:     task.ID,
			AttemptID:  attempt.ID,
			TaskStatus: string(attempt.Status),
			OK:         false,
			Error:      err.Error(),
		})
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attemptNo) * 150 * time.Millisecond):
		}
	}
	return lastErr
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return ""
	}
}

func buildTerminalNotificationText(task taskqueue.Task, attempt taskqueue.Attempt) string {
	title := strings.TrimSpace(task.Title)
	if title == "" {
		title = "Task"
	}
	lines := []string{
		fmt.Sprintf("%s (%s)", title, strings.TrimSpace(string(attempt.Status))),
		fmt.Sprintf("task_id: %s", strings.TrimSpace(task.ID)),
		fmt.Sprintf("attempt_id: %s", strings.TrimSpace(attempt.ID)),
	}
	if p := strings.TrimSpace(attempt.FindingsPath); p != "" {
		lines = append(lines, "findings_path: "+p)
	}
	if p := strings.TrimSpace(attempt.DiffPatchPath); p != "" {
		lines = append(lines, "diff_patch_path: "+p)
	}
	if p := strings.TrimSpace(attempt.ChangedFilesPath); p != "" {
		lines = append(lines, "changed_files_path: "+p)
	}
	if p := strings.TrimSpace(attempt.TestReportPath); p != "" {
		lines = append(lines, "test_report_path: "+p)
	}
	if p := strings.TrimSpace(attempt.TraceLogPath); p != "" {
		lines = append(lines, "trace_log_path: "+p)
	}
	return strings.Join(lines, "\n")
}
