//go:build linux

package shell

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunBashNative_BlocksMktempOutsideRoot(t *testing.T) {
	if _, err := landlockABIVersion(); err != nil {
		t.Skipf("landlock not available: %v", err)
	}
	if _, err := exec.LookPath("mktemp"); err != nil {
		t.Skipf("mktemp not found: %v", err)
	}

	root := t.TempDir()
	cmd := "TMPDIR= mktemp -t oneagent-native-sandbox-test"

	host, err := RunBash(context.Background(), cmd, 10*time.Second, root, "")
	if err != nil {
		t.Fatalf("RunBash: %v", err)
	}
	if host.ExitCode != 0 {
		t.Fatalf("expected host mktemp to succeed, exit=%d stderr=%q", host.ExitCode, host.Stderr)
	}

	created := strings.TrimSpace(host.Stdout)
	created = strings.SplitN(created, "\n", 2)[0]
	if created == "" {
		t.Fatalf("expected mktemp to output a path")
	}
	if !filepath.IsAbs(created) {
		t.Fatalf("expected absolute path, got %q", created)
	}
	if isWithinRoot(root, created) {
		t.Fatalf("expected created path outside root, got %q (root=%q)", created, root)
	}
	if _, err := os.Stat(created); err != nil {
		t.Fatalf("expected created file to exist, stat err=%v", err)
	}
	_ = os.Remove(created)

	native, err := RunBashNative(context.Background(), cmd, 10*time.Second, root, "")
	if err != nil {
		t.Fatalf("RunBashNative: %v", err)
	}
	if native.ExitCode == 0 {
		t.Fatalf("expected native mktemp to fail, stdout=%q stderr=%q", native.Stdout, native.Stderr)
	}
}

