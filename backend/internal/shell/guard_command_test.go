package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuardCommand_RejectsCdOutsideRoot(t *testing.T) {
	root, err := ResolveBashRoot(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveBashRoot: %v", err)
	}
	err = GuardCommand("cd ..", root)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "outside bash root") {
		t.Fatalf("expected outside root error, got %q", err.Error())
	}
}

func TestGuardCommand_RejectsVariableExpansion(t *testing.T) {
	root, err := ResolveBashRoot(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveBashRoot: %v", err)
	}
	err = GuardCommand("echo $HOME", root)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "substitution") && !strings.Contains(err.Error(), "variable") {
		t.Fatalf("expected substitution error, got %q", err.Error())
	}
}

func TestGuardCommand_RejectsHeredocRedirection(t *testing.T) {
	root, err := ResolveBashRoot(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveBashRoot: %v", err)
	}
	err = GuardCommand("cat <<EOF", root)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "heredoc") {
		t.Fatalf("expected heredoc error, got %q", err.Error())
	}
}

func TestGuardCommand_RejectsRedirectOutsideRoot(t *testing.T) {
	root, err := ResolveBashRoot(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveBashRoot: %v", err)
	}
	err = GuardCommand("echo hi > /etc/passwd", root)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "outside bash root") {
		t.Fatalf("expected outside root error, got %q", err.Error())
	}
}

func TestGuardCommand_AllowsRedirectToDevNull(t *testing.T) {
	root, err := ResolveBashRoot(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveBashRoot: %v", err)
	}
	if err := GuardCommand("echo hi > /dev/null", root); err != nil {
		t.Fatalf("expected dev/null redirect allowed, got %v", err)
	}
}

func TestGuardCommand_AllowsGrepPatternWithSlashAndAngleBrackets(t *testing.T) {
	root, err := ResolveBashRoot(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveBashRoot: %v", err)
	}
	if err := GuardCommand(`grep -c "</html>" a.txt`, root); err != nil {
		t.Fatalf("expected grep pattern allowed, got %v", err)
	}
}

func TestGuardCommand_AllowsRmInsideRoot(t *testing.T) {
	root, err := ResolveBashRoot(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveBashRoot: %v", err)
	}
	if err := GuardCommand("rm -rf ./tmp", root); err != nil {
		t.Fatalf("expected rm allowed, got %v", err)
	}
}

func TestGuardCommand_RejectsRmOutsideRoot(t *testing.T) {
	root, err := ResolveBashRoot(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveBashRoot: %v", err)
	}
	err = GuardCommand("rm -rf ../tmp", root)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "outside bash root") {
		t.Fatalf("expected outside root error, got %q", err.Error())
	}
}

func TestGuardCommand_RejectsRmSymlinkDereference(t *testing.T) {
	root, err := ResolveBashRoot(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveBashRoot: %v", err)
	}
	err = GuardCommand("rm -rfL ./tmp", root)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "dereference") {
		t.Fatalf("expected dereference error, got %q", err.Error())
	}
}

func TestGuardCommand_RejectsCdSymlinkOutsideRoot(t *testing.T) {
	root, err := ResolveBashRoot(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveBashRoot: %v", err)
	}
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	err = GuardCommand("cd escape && rm -rf ./tmp", root)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "resolves outside bash root") && !strings.Contains(err.Error(), "outside bash root") {
		t.Fatalf("expected outside root error, got %q", err.Error())
	}
}
