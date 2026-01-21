package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const (
	defaultTimeout = 10 * time.Second
	maxTimeout     = 30 * time.Second
	maxOutputBytes = 64 * 1024
)

var blacklistedShells = map[string]bool{
	"fish": true,
	"nu":   true,
}

// Result captures bash execution details.
type Result struct {
	Shell           string
	Stdout          string
	Stderr          string
	ExitCode        int
	Duration        time.Duration
	TimedOut        bool
	StdoutTruncated bool
	StderrTruncated bool
}

// ResolveBashPath finds a usable bash binary on macOS/Linux.
func ResolveBashPath() (string, error) {
	if runtime.GOOS == "windows" {
		return "", errors.New("bash is not supported on windows")
	}

	if explicit := strings.TrimSpace(os.Getenv("LYLE_BASH_PATH")); explicit != "" {
		if err := assertShellNotBlacklisted(explicit); err != nil {
			return "", err
		}
		if !isExecutable(explicit) {
			return "", fmt.Errorf("bash not found at: %s", explicit)
		}
		return explicit, nil
	}

	if bashPath, err := exec.LookPath("bash"); err == nil {
		return bashPath, nil
	}

	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{"/bin/bash", "/usr/bin/bash", "/opt/homebrew/bin/bash"}
	default:
		candidates = []string{"/bin/bash", "/usr/bin/bash", "/usr/local/bin/bash"}
	}

	for _, candidate := range candidates {
		if isExecutable(candidate) {
			return candidate, nil
		}
	}

	return "", errors.New("bash not found on this system")
}

// RunBash executes a bash command and returns output plus metadata.
func RunBash(ctx context.Context, command string, timeout time.Duration) (Result, error) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return Result{}, errors.New("command is required")
	}

	shellPath, err := ResolveBashPath()
	if err != nil {
		return Result{}, err
	}

	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if timeout > maxTimeout {
		timeout = maxTimeout
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, shellPath, "-lc", trimmed)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdoutBuf := &limitedBuffer{limit: maxOutputBytes}
	stderrBuf := &limitedBuffer{limit: maxOutputBytes}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	timedOut := runCtx.Err() == context.DeadlineExceeded
	if timedOut {
		killProcessGroup(cmd.Process)
	}

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		switch {
		case errors.As(runErr, &exitErr):
			exitCode = exitErr.ExitCode()
		case errors.Is(runErr, context.DeadlineExceeded), timedOut:
			exitCode = -1
		default:
			return Result{
				Shell:           shellPath,
				Stdout:          stdoutBuf.String(),
				Stderr:          stderrBuf.String(),
				ExitCode:        exitCode,
				Duration:        duration,
				TimedOut:        timedOut,
				StdoutTruncated: stdoutBuf.Truncated(),
				StderrTruncated: stderrBuf.Truncated(),
			}, runErr
		}
	}

	if timedOut && exitCode == 0 {
		exitCode = -1
	}

	return Result{
		Shell:           shellPath,
		Stdout:          stdoutBuf.String(),
		Stderr:          stderrBuf.String(),
		ExitCode:        exitCode,
		Duration:        duration,
		TimedOut:        timedOut,
		StdoutTruncated: stdoutBuf.Truncated(),
		StderrTruncated: stderrBuf.Truncated(),
	}, nil
}

func assertShellNotBlacklisted(shellPath string) error {
	base := strings.ToLower(filepath.Base(shellPath))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if blacklistedShells[base] {
		return fmt.Errorf("unsupported shell %q", base)
	}
	return nil
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0111 != 0
}

func killProcessGroup(proc *os.Process) {
	if proc == nil {
		return
	}
	_ = syscall.Kill(-proc.Pid, syscall.SIGKILL)
}

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
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

func (l *limitedBuffer) String() string {
	return l.buf.String()
}

func (l *limitedBuffer) Truncated() bool {
	return l.truncated
}
