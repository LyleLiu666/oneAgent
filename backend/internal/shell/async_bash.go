package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

const (
	defaultAsyncMaxRuntime = 10 * time.Minute
	maxAsyncMaxRuntime     = 30 * time.Minute
	maxAsyncPollWait       = 30 * time.Second
	asyncJobRetention      = 5 * time.Minute
)

type AsyncBashStatus string

const (
	AsyncBashStatusRunning   AsyncBashStatus = "running"
	AsyncBashStatusCompleted AsyncBashStatus = "completed"
	AsyncBashStatusFailed    AsyncBashStatus = "failed"
	AsyncBashStatusCanceled  AsyncBashStatus = "canceled"
	AsyncBashStatusTimedOut  AsyncBashStatus = "timed_out"
)

type AsyncBashPollResult struct {
	JobID           string         `json:"job_id"`
	Status          AsyncBashStatus `json:"status"`
	Command         string         `json:"command"`
	Shell           string         `json:"shell"`
	StdoutDelta     string         `json:"stdout_delta,omitempty"`
	StderrDelta     string         `json:"stderr_delta,omitempty"`
	StdoutOffset    int            `json:"stdout_offset"`
	StderrOffset    int            `json:"stderr_offset"`
	ExitCode        int            `json:"exit_code,omitempty"`
	TimedOut        bool           `json:"timed_out,omitempty"`
	Canceled        bool           `json:"canceled,omitempty"`
	DurationMs      int64          `json:"duration_ms,omitempty"`
	ElapsedMs       int64          `json:"elapsed_ms"`
	StdoutTruncated bool           `json:"stdout_truncated"`
	StderrTruncated bool           `json:"stderr_truncated"`
}

type asyncLimitedBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (l *asyncLimitedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.limit <= 0 {
		l.truncated = true
		return len(p), nil
	}

	if l.buf.Len()+len(p) <= l.limit {
		return l.buf.Write(p)
	}

	remaining := l.limit - l.buf.Len()
	if remaining > 0 {
		_, _ = l.buf.Write(p[:remaining])
	}
	l.truncated = true
	return len(p), nil
}

func (l *asyncLimitedBuffer) snapshot() ([]byte, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	data := make([]byte, l.buf.Len())
	copy(data, l.buf.Bytes())
	return data, l.truncated
}

type asyncBashJob struct {
	id         string
	command    string
	shell      string
	startedAt  time.Time
	maxRuntime time.Duration
	cmd        *exec.Cmd
	stdout     *asyncLimitedBuffer
	stderr     *asyncLimitedBuffer

	doneCh chan struct{}

	mu       sync.Mutex
	done     bool
	timedOut bool
	canceled bool
	exitCode int
	duration time.Duration
}

type asyncBashManager struct {
	mu   sync.Mutex
	jobs map[string]*asyncBashJob
}

func newAsyncBashManager() *asyncBashManager {
	return &asyncBashManager{
		jobs: make(map[string]*asyncBashJob),
	}
}

var defaultAsyncBashManager = newAsyncBashManager()

func StartBashAsync(command string, maxRuntime time.Duration, rootDir string) (string, error) {
	return defaultAsyncBashManager.start(command, maxRuntime, rootDir)
}

func PollBashAsync(ctx context.Context, jobID string, wait time.Duration, stdoutOffset, stderrOffset int) (AsyncBashPollResult, error) {
	return defaultAsyncBashManager.poll(ctx, jobID, wait, stdoutOffset, stderrOffset)
}

func CancelBashAsync(jobID string) error {
	return defaultAsyncBashManager.cancel(jobID)
}

func (m *asyncBashManager) get(jobID string) (*asyncBashJob, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[jobID]
	return job, ok
}

func (m *asyncBashManager) delete(jobID string) {
	m.mu.Lock()
	delete(m.jobs, jobID)
	m.mu.Unlock()
}

func (m *asyncBashManager) start(command string, maxRuntime time.Duration, rootDir string) (string, error) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return "", errors.New("command is required")
	}

	root, err := ResolveBashRoot(rootDir)
	if err != nil {
		return "", err
	}

	if err := GuardCommand(trimmed, root); err != nil {
		return "", err
	}

	shellPath, err := ResolveBashPath()
	if err != nil {
		return "", err
	}

	if maxRuntime <= 0 {
		maxRuntime = defaultAsyncMaxRuntime
	}
	if maxRuntime > maxAsyncMaxRuntime {
		maxRuntime = maxAsyncMaxRuntime
	}

	tmpDir := filepath.Join(root, tmpDirName)
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return "", fmt.Errorf("failed to prepare bash temp dir: %w", err)
	}

	stdoutBuf := &asyncLimitedBuffer{limit: maxOutputBytes}
	stderrBuf := &asyncLimitedBuffer{limit: maxOutputBytes}

	cmd := exec.Command(shellPath, "--noprofile", "--norc", "-lc", trimmed)
	cmd.Dir = root
	cmd.Env = mergeEnv(os.Environ(), map[string]string{
		"HOME":          root,
		"PWD":           root,
		"BASH_ROOT_DIR": root,
		"TMPDIR":        tmpDir,
		"TMP":           tmpDir,
		"TEMP":          tmpDir,
		"BASH_ENV":      "",
	})
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		return "", err
	}

	jobID := uuid.NewString()
	job := &asyncBashJob{
		id:         jobID,
		command:    trimmed,
		shell:      shellPath,
		startedAt:  time.Now(),
		maxRuntime: maxRuntime,
		cmd:        cmd,
		stdout:     stdoutBuf,
		stderr:     stderrBuf,
		doneCh:     make(chan struct{}),
	}

	m.mu.Lock()
	m.jobs[jobID] = job
	m.mu.Unlock()

	go job.wait(m)

	return jobID, nil
}

func (job *asyncBashJob) wait(m *asyncBashManager) {
	defer func() {
		time.AfterFunc(asyncJobRetention, func() { m.delete(job.id) })
	}()

	waitCh := make(chan error, 1)
	go func() { waitCh <- job.cmd.Wait() }()

	timer := time.NewTimer(job.maxRuntime)
	defer timer.Stop()

	var (
		waitErr  error
		timedOut bool
	)

	select {
	case waitErr = <-waitCh:
		// completed
	case <-timer.C:
		timedOut = true
		killProcessGroup(job.cmd.Process)
		waitErr = <-waitCh
	}

	exitCode := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	if timedOut {
		exitCode = -1
	}

	job.mu.Lock()
	job.done = true
	job.timedOut = timedOut
	job.exitCode = exitCode
	job.duration = time.Since(job.startedAt)
	job.mu.Unlock()

	close(job.doneCh)
}

func (m *asyncBashManager) cancel(jobID string) error {
	job, ok := m.get(jobID)
	if !ok {
		return fmt.Errorf("unknown job_id: %s", jobID)
	}

	job.mu.Lock()
	if job.done {
		job.mu.Unlock()
		return nil
	}
	job.canceled = true
	proc := job.cmd.Process
	job.mu.Unlock()

	killProcessGroup(proc)
	return nil
}

func (m *asyncBashManager) poll(ctx context.Context, jobID string, wait time.Duration, stdoutOffset, stderrOffset int) (AsyncBashPollResult, error) {
	job, ok := m.get(jobID)
	if !ok {
		return AsyncBashPollResult{}, fmt.Errorf("unknown job_id: %s", jobID)
	}

	if wait > 0 {
		if wait > maxAsyncPollWait {
			wait = maxAsyncPollWait
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return AsyncBashPollResult{}, ctx.Err()
		case <-job.doneCh:
			timer.Stop()
		case <-timer.C:
		}
	}

	stdoutBytes, stdoutTruncated := job.stdout.snapshot()
	stderrBytes, stderrTruncated := job.stderr.snapshot()

	if stdoutOffset < 0 {
		stdoutOffset = 0
	}
	if stderrOffset < 0 {
		stderrOffset = 0
	}
	if stdoutOffset > len(stdoutBytes) {
		stdoutOffset = len(stdoutBytes)
	}
	if stderrOffset > len(stderrBytes) {
		stderrOffset = len(stderrBytes)
	}

	job.mu.Lock()
	done := job.done
	timedOut := job.timedOut
	canceled := job.canceled
	exitCode := job.exitCode
	duration := job.duration
	startedAt := job.startedAt
	command := job.command
	shellPath := job.shell
	job.mu.Unlock()

	elapsed := time.Since(startedAt)

	status := AsyncBashStatusRunning
	if done {
		switch {
		case timedOut:
			status = AsyncBashStatusTimedOut
		case canceled:
			status = AsyncBashStatusCanceled
		case exitCode == 0:
			status = AsyncBashStatusCompleted
		default:
			status = AsyncBashStatusFailed
		}
	}

	result := AsyncBashPollResult{
		JobID:           job.id,
		Status:          status,
		Command:         command,
		Shell:           shellPath,
		StdoutDelta:     string(stdoutBytes[stdoutOffset:]),
		StderrDelta:     string(stderrBytes[stderrOffset:]),
		StdoutOffset:    len(stdoutBytes),
		StderrOffset:    len(stderrBytes),
		ExitCode:        exitCode,
		TimedOut:        timedOut,
		Canceled:        canceled,
		ElapsedMs:       elapsed.Milliseconds(),
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: stderrTruncated,
	}

	if done {
		result.DurationMs = duration.Milliseconds()
	}

	return result, nil
}

