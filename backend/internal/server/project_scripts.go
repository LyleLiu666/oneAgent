package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/subagent"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func runCopyFilesChecks(logPath string, workspaceRoot string, copyFiles []string) error {
	return runCopyFiles(logPath, workspaceRoot, workspaceRoot, copyFiles)
}

func runCopyFiles(logPath string, sourceRoot string, destRoot string, copyFiles []string) error {
	sourceRoot = strings.TrimSpace(sourceRoot)
	if sourceRoot == "" {
		return errors.New("source_root is required")
	}
	destRoot = strings.TrimSpace(destRoot)
	if destRoot == "" {
		return errors.New("dest_root is required")
	}
	logPath = strings.TrimSpace(logPath)
	if logPath == "" {
		return errors.New("logPath is required")
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return fmt.Errorf("prepare copy_files log dir: %w", err)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open copy_files log: %w", err)
	}
	defer f.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	fmt.Fprintf(f, "# copy_files\n\n- checked_at: %s\n- source_root: %s\n- dest_root: %s\n\n", now, sourceRoot, destRoot)

	if len(copyFiles) == 0 {
		fmt.Fprintln(f, "- copy_files: (empty)")
		fmt.Fprintln(f)
		return nil
	}

	for _, rel := range copyFiles {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		srcAbs := filepath.Join(sourceRoot, filepath.FromSlash(rel))
		if st, err := os.Stat(srcAbs); err != nil {
			fmt.Fprintf(f, "- %s: missing (%v)\n", rel, err)
			return fmt.Errorf("copy_files missing: %s", rel)
		} else if st.IsDir() {
			fmt.Fprintf(f, "- %s: is a directory\n", rel)
			return fmt.Errorf("copy_files is a directory: %s", rel)
		}

		if sourceRoot != destRoot {
			dstAbs := filepath.Join(destRoot, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(dstAbs), 0o700); err != nil {
				fmt.Fprintf(f, "- %s: copy failed (mkdir) (%v)\n", rel, err)
				return fmt.Errorf("copy_files mkdir: %s", rel)
			}
			src, err := os.Open(srcAbs)
			if err != nil {
				fmt.Fprintf(f, "- %s: copy failed (open) (%v)\n", rel, err)
				return fmt.Errorf("copy_files open: %s", rel)
			}
			dst, err := os.OpenFile(dstAbs, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				_ = src.Close()
				fmt.Fprintf(f, "- %s: copy failed (create) (%v)\n", rel, err)
				return fmt.Errorf("copy_files create: %s", rel)
			}
			if _, err := io.Copy(dst, src); err != nil {
				_ = dst.Close()
				_ = src.Close()
				fmt.Fprintf(f, "- %s: copy failed (write) (%v)\n", rel, err)
				return fmt.Errorf("copy_files write: %s", rel)
			}
			_ = dst.Close()
			_ = src.Close()
			fmt.Fprintf(f, "- %s: copied\n", rel)
			continue
		}

		fmt.Fprintf(f, "- %s: ok\n", rel)
	}
	fmt.Fprintln(f)
	return nil
}

func runProjectScript(ctx context.Context, handlers map[string]subagent.ToolHandler, scriptName string, command string, logPath string) error {
	scriptName = strings.TrimSpace(scriptName)
	command = strings.TrimSpace(command)
	logPath = strings.TrimSpace(logPath)

	if scriptName == "" {
		return errors.New("scriptName is required")
	}
	if command == "" {
		return errors.New("command is required")
	}
	if logPath == "" {
		return errors.New("logPath is required")
	}
	if handlers == nil {
		return errors.New("handlers is required")
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return fmt.Errorf("prepare script log dir: %w", err)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open script log: %w", err)
	}
	defer f.Close()

	startedAt := time.Now().UTC()
	fmt.Fprintf(f, "# %s\n\n- started_at: %s\n- command: %s\n\n", scriptName, startedAt.Format(time.RFC3339), command)

	if h, ok := handlers["run_command"]; ok {
		err := runScriptViaRunCommand(ctx, h, f, scriptName, command)
		writeScriptFooter(f, scriptName, startedAt, err)
		return err
	}
	if h, ok := handlers["bash"]; ok {
		err := runScriptViaBash(ctx, h, f, scriptName, command)
		writeScriptFooter(f, scriptName, startedAt, err)
		return err
	}

	err = errors.New("no command tool available (need run_command or bash)")
	writeScriptFooter(f, scriptName, startedAt, err)
	return err
}

func writeScriptFooter(f *os.File, scriptName string, startedAt time.Time, runErr error) {
	if f == nil {
		return
	}
	status := "succeeded"
	errMsg := ""
	if runErr != nil {
		status = "failed"
		errMsg = runErr.Error()
	}
	fmt.Fprintf(f, "\n\n## Result\n\n- status: %s\n- duration_ms: %d\n", status, time.Since(startedAt).Milliseconds())
	if errMsg != "" {
		fmt.Fprintf(f, "- error: %s\n", errMsg)
	}
}

func runScriptViaBash(ctx context.Context, h subagent.ToolHandler, logFile *os.File, scriptName, command string) error {
	raw, _ := json.Marshal(map[string]any{
		"command":    command,
		"timeout_ms": int((30 * time.Second).Milliseconds()),
	})
	out, err := h(ctx, raw)
	if err != nil {
		fmt.Fprintf(logFile, "## ToolError\n\n%v\n", err)
		return err
	}

	var res tool.BashToolResult
	switch v := out.(type) {
	case tool.BashToolResult:
		res = v
	default:
		b, _ := json.Marshal(out)
		_ = json.Unmarshal(b, &res)
	}

	if strings.TrimSpace(res.Stdout) != "" {
		fmt.Fprintf(logFile, "## stdout\n\n%s\n", res.Stdout)
	}
	if strings.TrimSpace(res.Stderr) != "" {
		fmt.Fprintf(logFile, "## stderr\n\n%s\n", res.Stderr)
	}
	fmt.Fprintf(logFile, "\n- exit_code: %d\n- timed_out: %t\n", res.ExitCode, res.TimedOut)

	if res.TimedOut {
		return fmt.Errorf("%s timed out", scriptName)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("%s exit_code=%d", scriptName, res.ExitCode)
	}
	return nil
}

func runScriptViaRunCommand(ctx context.Context, h subagent.ToolHandler, logFile *os.File, scriptName, command string) error {
	jobID := ""
	stdoutOffset := 0
	stderrOffset := 0
	action := "start"

	for {
		req := map[string]any{
			"action":              action,
			"wait_seconds":        30,
			"max_runtime_seconds": 1800,
			"stdout_offset":       stdoutOffset,
			"stderr_offset":       stderrOffset,
			"max_delta_bytes":     65536,
		}
		if action == "start" {
			req["command"] = command
		} else {
			req["job_id"] = jobID
		}
		raw, _ := json.Marshal(req)
		out, err := h(ctx, raw)
		if err != nil {
			fmt.Fprintf(logFile, "## ToolError\n\n%v\n", err)
			return err
		}

		var res tool.RunCommandResult
		switch v := out.(type) {
		case tool.RunCommandResult:
			res = v
		default:
			b, _ := json.Marshal(out)
			_ = json.Unmarshal(b, &res)
		}

		if jobID == "" {
			jobID = strings.TrimSpace(res.JobID)
		}

		if strings.TrimSpace(res.StdoutDelta) != "" {
			fmt.Fprintf(logFile, "## stdout\n\n%s\n", res.StdoutDelta)
		}
		if strings.TrimSpace(res.StderrDelta) != "" {
			fmt.Fprintf(logFile, "## stderr\n\n%s\n", res.StderrDelta)
		}
		stdoutOffset = res.StdoutOffset
		stderrOffset = res.StderrOffset

		switch res.Status {
		case tool.RunCommandStatusRunning, tool.RunCommandStatusCanceling:
			action = "poll"
			continue
		case tool.RunCommandStatusCompleted:
			fmt.Fprintf(logFile, "\n- exit_code: %d\n", res.ExitCode)
			if res.ExitCode != 0 {
				return fmt.Errorf("%s exit_code=%d", scriptName, res.ExitCode)
			}
			return nil
		case tool.RunCommandStatusFailed:
			fmt.Fprintf(logFile, "\n- exit_code: %d\n", res.ExitCode)
			if res.ExitCode != 0 {
				return fmt.Errorf("%s exit_code=%d", scriptName, res.ExitCode)
			}
			return fmt.Errorf("%s failed", scriptName)
		case tool.RunCommandStatusCanceled:
			return fmt.Errorf("%s canceled", scriptName)
		case tool.RunCommandStatusTimedOut:
			return fmt.Errorf("%s timed out", scriptName)
		default:
			return fmt.Errorf("%s: unexpected run_command status: %s", scriptName, res.Status)
		}
	}
}
