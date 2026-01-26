package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultAsyncMaxRuntime = 10 * time.Minute
	maxAsyncMaxRuntime     = 30 * time.Minute
	maxAsyncPollWait       = 30 * time.Second
	asyncJobRetention      = 30 * time.Minute
	asyncLogCapacityBytes  = 2 * 1024 * 1024

	defaultAsyncMaxDeltaBytes = 16 * 1024
	maxAsyncMaxDeltaBytes     = 64 * 1024
)

type AsyncBashStatus string

const (
	AsyncBashStatusRunning   AsyncBashStatus = "running"
	AsyncBashStatusCanceling AsyncBashStatus = "canceling"
	AsyncBashStatusCompleted AsyncBashStatus = "completed"
	AsyncBashStatusFailed    AsyncBashStatus = "failed"
	AsyncBashStatusCanceled  AsyncBashStatus = "canceled"
	AsyncBashStatusTimedOut  AsyncBashStatus = "timed_out"
)

type AsyncBashPollResult struct {
	JobID            string          `json:"job_id"`
	Status           AsyncBashStatus `json:"status"`
	Command          string          `json:"command"`
	Shell            string          `json:"shell"`
	StdoutDelta      string          `json:"stdout_delta,omitempty"`
	StderrDelta      string          `json:"stderr_delta,omitempty"`
	StdoutBaseOffset int             `json:"stdout_base_offset"`
	StderrBaseOffset int             `json:"stderr_base_offset"`
	StdoutOffset     int             `json:"stdout_offset"`
	StderrOffset     int             `json:"stderr_offset"`
	ExitCode         int             `json:"exit_code,omitempty"`
	TimedOut         bool            `json:"timed_out,omitempty"`
	Canceled         bool            `json:"canceled,omitempty"`
	DurationMs       int64           `json:"duration_ms,omitempty"`
	ElapsedMs        int64           `json:"elapsed_ms"`
	StdoutTruncated  bool            `json:"stdout_truncated"`
	StderrTruncated  bool            `json:"stderr_truncated"`
}

type ringLog struct {
	mu          sync.Mutex
	path        string
	f           *os.File
	capacity    int
	size        int
	writePos    int
	totalOffset int64
	notifyCh    chan struct{}
}

func newRingLog(path string, capacity int, notifyCh chan struct{}) (*ringLog, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("invalid ring log capacity: %d", capacity)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Truncate(int64(capacity)); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &ringLog{
		path:     path,
		f:        f,
		capacity: capacity,
		notifyCh: notifyCh,
	}, nil
}

func (l *ringLog) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}

func (l *ringLog) notifyLocked() {
	if l.notifyCh == nil {
		return
	}
	select {
	case l.notifyCh <- struct{}{}:
	default:
	}
}

func (l *ringLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.f == nil {
		return 0, errors.New("log is closed")
	}

	n := len(p)
	if n == 0 {
		return 0, nil
	}

	if n >= l.capacity {
		tail := p[n-l.capacity:]
		if _, err := l.f.WriteAt(tail, 0); err != nil {
			return 0, err
		}
		l.size = l.capacity
		l.writePos = 0
		l.totalOffset += int64(n)
		l.notifyLocked()
		return n, nil
	}

	remaining := n
	cursor := 0
	for remaining > 0 {
		chunk := remaining
		space := l.capacity - l.writePos
		if chunk > space {
			chunk = space
		}
		if _, err := l.f.WriteAt(p[cursor:cursor+chunk], int64(l.writePos)); err != nil {
			return 0, err
		}
		l.writePos = (l.writePos + chunk) % l.capacity
		cursor += chunk
		remaining -= chunk
	}

	if l.size < l.capacity {
		l.size += n
		if l.size > l.capacity {
			l.size = l.capacity
		}
	}
	l.totalOffset += int64(n)
	l.notifyLocked()
	return n, nil
}

func (l *ringLog) readSince(offset, maxBytes int) ([]byte, int, int, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var readFile *os.File
	if l.f != nil {
		readFile = l.f
	} else if l.path != "" {
		f, err := os.Open(l.path)
		if err != nil {
			return nil, 0, 0, false, err
		}
		defer f.Close()
		readFile = f
	} else {
		return nil, 0, 0, false, errors.New("log file is unavailable")
	}

	baseOffset := int(l.totalOffset) - l.size
	if baseOffset < 0 {
		baseOffset = 0
	}

	if offset < baseOffset {
		offset = baseOffset
	}
	if offset > int(l.totalOffset) {
		offset = int(l.totalOffset)
	}

	available := int(l.totalOffset) - offset
	if available <= 0 {
		return nil, offset, baseOffset, baseOffset > 0, nil
	}

	if maxBytes <= 0 {
		maxBytes = defaultAsyncMaxDeltaBytes
	}
	if maxBytes > maxAsyncMaxDeltaBytes {
		maxBytes = maxAsyncMaxDeltaBytes
	}

	toRead := available
	if toRead > maxBytes {
		toRead = maxBytes
	}

	startRel := offset - baseOffset
	startPos := startRel
	if l.size == l.capacity {
		startPos = (l.writePos + startRel) % l.capacity
	}

	delta := make([]byte, toRead)
	first := toRead
	space := l.capacity - startPos
	if first > space {
		first = space
	}
	if _, err := readFile.ReadAt(delta[:first], int64(startPos)); err != nil {
		return nil, offset, baseOffset, baseOffset > 0, err
	}
	if first < toRead {
		if _, err := readFile.ReadAt(delta[first:], 0); err != nil {
			return nil, offset, baseOffset, baseOffset > 0, err
		}
	}

	newOffset := offset + toRead
	return delta, newOffset, baseOffset, baseOffset > 0, nil
}

type asyncBashJob struct {
	id         string
	command    string
	shell      string
	startedAt  time.Time
	maxRuntime time.Duration
	cmd        *exec.Cmd
	stdout     *ringLog
	stderr     *ringLog
	notifyCh   chan struct{}
	jobDir     string

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

func PollBashAsync(ctx context.Context, jobID string, wait time.Duration, stdoutOffset, stderrOffset, maxDeltaBytes int) (AsyncBashPollResult, error) {
	return defaultAsyncBashManager.poll(ctx, jobID, wait, stdoutOffset, stderrOffset, maxDeltaBytes)
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
	job := m.jobs[jobID]
	delete(m.jobs, jobID)
	m.mu.Unlock()

	if job == nil {
		return
	}
	if job.stdout != nil {
		_ = job.stdout.Close()
	}
	if job.stderr != nil {
		_ = job.stderr.Close()
	}
	if job.jobDir != "" {
		_ = os.RemoveAll(job.jobDir)
	}
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

	jobID := uuid.NewString()
	jobDir := filepath.Join(tmpDir, "run_command", jobID)
	if err := os.MkdirAll(jobDir, 0o700); err != nil {
		return "", fmt.Errorf("failed to prepare run_command dir: %w", err)
	}

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
	setupCmdForProcessGroup(cmd)

	notifyCh := make(chan struct{}, 1)
	stdoutLog, err := newRingLog(filepath.Join(jobDir, "stdout.log"), asyncLogCapacityBytes, notifyCh)
	if err != nil {
		_ = os.RemoveAll(jobDir)
		return "", err
	}
	stderrLog, err := newRingLog(filepath.Join(jobDir, "stderr.log"), asyncLogCapacityBytes, notifyCh)
	if err != nil {
		_ = stdoutLog.Close()
		_ = os.RemoveAll(jobDir)
		return "", err
	}

	cmd.Stdout = stdoutLog
	cmd.Stderr = stderrLog

	if err := cmd.Start(); err != nil {
		_ = stdoutLog.Close()
		_ = stderrLog.Close()
		_ = os.RemoveAll(jobDir)
		return "", err
	}

	job := &asyncBashJob{
		id:         jobID,
		command:    trimmed,
		shell:      shellPath,
		startedAt:  time.Now(),
		maxRuntime: maxRuntime,
		cmd:        cmd,
		stdout:     stdoutLog,
		stderr:     stderrLog,
		notifyCh:   notifyCh,
		jobDir:     jobDir,
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
		killProcessTree(job.cmd.Process)
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

	if job.stdout != nil {
		_ = job.stdout.Close()
	}
	if job.stderr != nil {
		_ = job.stderr.Close()
	}

	close(job.doneCh)
}

func (m *asyncBashManager) cancel(jobID string) error {
	job, ok := m.get(jobID)
	if !ok {
		return fmt.Errorf("unknown job_id: %s（可能已过期或服务已重启）", jobID)
	}

	job.mu.Lock()
	if job.done {
		job.mu.Unlock()
		return nil
	}
	job.canceled = true
	proc := job.cmd.Process
	job.mu.Unlock()

	killProcessTree(proc)
	select {
	case job.notifyCh <- struct{}{}:
	default:
	}
	return nil
}

func (m *asyncBashManager) poll(ctx context.Context, jobID string, wait time.Duration, stdoutOffset, stderrOffset, maxDeltaBytes int) (AsyncBashPollResult, error) {
	job, ok := m.get(jobID)
	if !ok {
		return AsyncBashPollResult{}, fmt.Errorf("unknown job_id: %s（可能已过期或服务已重启）", jobID)
	}

	if wait > 0 {
		if wait > maxAsyncPollWait {
			wait = maxAsyncPollWait
		}
	}

	if maxDeltaBytes <= 0 {
		maxDeltaBytes = defaultAsyncMaxDeltaBytes
	}
	if maxDeltaBytes > maxAsyncMaxDeltaBytes {
		maxDeltaBytes = maxAsyncMaxDeltaBytes
	}

	deadline := time.Now().Add(wait)
	for {
		job.mu.Lock()
		done := job.done
		timedOut := job.timedOut
		canceled := job.canceled
		exitCode := job.exitCode
		duration := job.duration
		startedAt := job.startedAt
		command := job.command
		shellPath := job.shell
		notifyCh := job.notifyCh
		job.mu.Unlock()

		stdoutBytes, newStdoutOffset, stdoutBaseOffset, stdoutTruncated, err := job.stdout.readSince(stdoutOffset, maxDeltaBytes)
		if err != nil {
			return AsyncBashPollResult{}, err
		}
		stderrBytes, newStderrOffset, stderrBaseOffset, stderrTruncated, err := job.stderr.readSince(stderrOffset, maxDeltaBytes)
		if err != nil {
			return AsyncBashPollResult{}, err
		}

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
		} else if canceled {
			status = AsyncBashStatusCanceling
		}

		result := AsyncBashPollResult{
			JobID:            job.id,
			Status:           status,
			Command:          command,
			Shell:            shellPath,
			StdoutDelta:      string(stdoutBytes),
			StderrDelta:      string(stderrBytes),
			StdoutBaseOffset: stdoutBaseOffset,
			StderrBaseOffset: stderrBaseOffset,
			StdoutOffset:     newStdoutOffset,
			StderrOffset:     newStderrOffset,
			ExitCode:         exitCode,
			TimedOut:         timedOut,
			Canceled:         canceled,
			ElapsedMs:        elapsed.Milliseconds(),
			StdoutTruncated:  stdoutTruncated,
			StderrTruncated:  stderrTruncated,
		}

		if done {
			result.DurationMs = duration.Milliseconds()
		}

		if wait <= 0 || done || len(stdoutBytes) > 0 || len(stderrBytes) > 0 {
			return result, nil
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return result, nil
		}

		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			return AsyncBashPollResult{}, ctx.Err()
		case <-job.doneCh:
			timer.Stop()
		case <-notifyCh:
			timer.Stop()
		case <-timer.C:
		}
	}
}
