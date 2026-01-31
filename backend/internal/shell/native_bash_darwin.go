//go:build darwin

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

func RunBashNative(ctx context.Context, command string, timeout time.Duration, rootDir string, workDir string) (Result, error) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return Result{}, errors.New("command is required")
	}

	root, err := ResolveBashRoot(rootDir)
	if err != nil {
		return Result{}, err
	}

	if err := GuardCommand(trimmed, root); err != nil {
		return Result{}, err
	}

	startDir := root
	if workDir != "" {
		if err := ensurePathWithinRoot(root, workDir); err != nil {
			return Result{}, fmt.Errorf("invalid working directory: %w", err)
		}
		startDir = workDir
	}

	sandboxExecPath, err := exec.LookPath("sandbox-exec")
	if err != nil {
		return Result{}, fmt.Errorf("native sandbox unavailable: sandbox-exec not found (expected /usr/bin/sandbox-exec)")
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

	tmpDir := filepath.Join(root, tmpDirName)
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("failed to prepare bash temp dir: %w", err)
	}

	profilePath, err := writeNativeSandboxProfile(tmpDir, root)
	if err != nil {
		return Result{}, err
	}
	defer os.Remove(profilePath)

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	const pwdMarker = "ONEAGENT_PWD_MARKER="
	wrappedCmd := fmt.Sprintf("%s\nRET=$?\necho\necho %s$PWD\nexit $RET", trimmed, pwdMarker)

	cmd := exec.CommandContext(runCtx, sandboxExecPath, "-f", profilePath, shellPath, "--noprofile", "--norc", "-lc", wrappedCmd)
	cmd.Dir = startDir
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
				Shell:           "native",
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

	// Parse CWD from stdout.
	stdoutStr := stdoutBuf.String()
	cwd := startDir
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
		Shell:           "native",
		Stdout:          stdoutStr,
		Stderr:          stderrBuf.String(),
		ExitCode:        exitCode,
		Duration:        duration,
		TimedOut:        timedOut,
		StdoutTruncated: stdoutBuf.Truncated(),
		StderrTruncated: stderrBuf.Truncated(),
		CWD:             cwd,
	}, nil
}

func writeNativeSandboxProfile(dir string, root string) (string, error) {
	escapedRoot := strings.ReplaceAll(root, `"`, `\"`)
	profile := fmt.Sprintf(`(version 1)
(deny default)
(allow process*)
(allow network*)
(allow sysctl-read)
(allow file-read* file-map-executable)
(allow file-write*
  (subpath "%s")
  (literal "/dev/null"))`, escapedRoot)

	f, err := os.CreateTemp(dir, "oneagent-native-sandbox-*.sb")
	if err != nil {
		return "", fmt.Errorf("failed to create native sandbox profile: %w", err)
	}
	path := f.Name()
	if _, err := f.WriteString(profile); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("failed to write native sandbox profile: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("failed to close native sandbox profile: %w", err)
	}
	return path, nil
}

