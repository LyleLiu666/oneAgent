package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const defaultDockerSandboxImage = "ubuntu:22.04"

func dockerSandboxImage() string {
	if v := strings.TrimSpace(os.Getenv("ONEAGENT_DOCKER_SANDBOX_IMAGE")); v != "" {
		return v
	}
	return defaultDockerSandboxImage
}

func ensureDockerAvailable() (string, error) {
	dockerPath, err := exec.LookPath("docker")
	if err == nil {
		return dockerPath, nil
	}
	return "", errors.New("docker is required for sandbox_mode=docker (install Docker Desktop or set sandbox_mode=none)")
}

func rejectAbsolutePathsForDocker(command string) error {
	tokens, err := splitCommandTokens(command)
	if err != nil {
		return &UnsafeCommandError{Reason: err.Error()}
	}
	for _, token := range tokens {
		if token.kind != tokenWord {
			continue
		}
		v := strings.TrimSpace(token.value)
		if v == "" {
			continue
		}
		if v == "/dev/null" {
			continue
		}
		if strings.HasPrefix(v, "/") {
			return &UnsafeCommandError{Reason: "absolute paths are not allowed in docker sandbox; use relative paths within the workspace root"}
		}
	}
	return nil
}

func RunBashDocker(ctx context.Context, command string, timeout time.Duration, rootDir string) (Result, error) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return Result{}, errors.New("command is required")
	}

	root, err := ResolveBashRoot(rootDir)
	if err != nil {
		return Result{}, err
	}

	if err := rejectAbsolutePathsForDocker(trimmed); err != nil {
		return Result{}, err
	}
	if err := GuardCommand(trimmed, root); err != nil {
		return Result{}, err
	}

	dockerPath, err := ensureDockerAvailable()
	if err != nil {
		return Result{}, err
	}

	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if timeout > maxTimeout {
		timeout = maxTimeout
	}

	tmpDir := filepath.Join(root, tmpDirName)
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("failed to prepare bash temp dir: %w", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	const pwdMarker = "ONEAGENT_PWD_MARKER="
	wrappedCmd := fmt.Sprintf("%s\nRET=$?\necho\necho %s$PWD\nexit $RET", trimmed, pwdMarker)

	args := []string{
		"run",
		"--rm",
		"--network", "none",
		"-v", fmt.Sprintf("%s:/workspace", root),
		"-w", "/workspace",
		"-e", "HOME=/workspace",
		"-e", "PWD=/workspace",
		"-e", "BASH_ROOT_DIR=/workspace",
		"-e", "BASH_ENV=",
		dockerSandboxImage(),
		"bash", "--noprofile", "--norc", "-lc", wrappedCmd,
	}

	cmd := exec.CommandContext(runCtx, dockerPath, args...)
	cmd.Dir = root
	cmd.Env = os.Environ()
	setupCmdForProcessGroup(cmd)

	stdoutBuf := &limitedBuffer{limit: maxOutputBytes}
	stderrBuf := &limitedBuffer{limit: maxOutputBytes}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	timedOut := runCtx.Err() == context.DeadlineExceeded
	if timedOut {
		killProcessTree(cmd.Process)
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
				Shell:           "docker",
				Stdout:          stdoutBuf.String(),
				Stderr:          stderrBuf.String(),
				ExitCode:        exitCode,
				Duration:        duration,
				TimedOut:        timedOut,
				StdoutTruncated: stdoutBuf.Truncated(),
				StderrTruncated: stderrBuf.Truncated(),
				CWD:             "/workspace",
			}, runErr
		}
	}

	if timedOut && exitCode == 0 {
		exitCode = -1
	}

	stdoutStr := stdoutBuf.String()
	cwd := "/workspace"
	if idx := strings.LastIndex(stdoutStr, pwdMarker); idx >= 0 {
		line := stdoutStr[idx+len(pwdMarker):]
		cwd = strings.TrimSpace(line)
		cutPoint := idx
		if cutPoint > 0 && stdoutStr[cutPoint-1] == '\n' {
			cutPoint--
		}
		stdoutStr = stdoutStr[:cutPoint]
	}

	return Result{
		Shell:           "docker",
		Stdout:          strings.TrimSpace(stdoutStr),
		Stderr:          strings.TrimSpace(stderrBuf.String()),
		ExitCode:        exitCode,
		Duration:        duration,
		TimedOut:        timedOut,
		StdoutTruncated: stdoutBuf.Truncated(),
		StderrTruncated: stderrBuf.Truncated(),
		CWD:             cwd,
	}, nil
}
