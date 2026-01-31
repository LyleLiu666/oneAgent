package shell

import (
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
