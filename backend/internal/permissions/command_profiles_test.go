package permissions

import "testing"

func TestValidateCommand_Readonly(t *testing.T) {
	if err := ValidateCommand("readonly", "ls -la", nil); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
	if err := ValidateCommand("readonly", "python -V", nil); err == nil {
		t.Fatalf("expected deny for python")
	}
}

func TestValidateCommand_Readonly_DeniesRedirection(t *testing.T) {
	if err := ValidateCommand("readonly", "echo hi > out.txt", nil); err == nil {
		t.Fatalf("expected deny for redirection")
	}
	if err := ValidateCommand("readonly", `echo ">"`, nil); err != nil {
		t.Fatalf("expected allow for quoted >, got %v", err)
	}
}

func TestValidateCommand_Dev_DeniesInterpretersInChains(t *testing.T) {
	cases := []string{
		"echo A; python -V",
		"echo A;python -V",
		"echo A && python -V",
		"echo A || python -V",
		"echo A | python -V",
	}
	for _, cmd := range cases {
		if err := ValidateCommand("dev", cmd, nil); err == nil {
			t.Fatalf("expected deny for %q", cmd)
		}
	}
}

func TestValidateCommand_Dev_AllowsCd(t *testing.T) {
	if err := ValidateCommand("dev", "cd . && ls", nil); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
}

func TestValidateCommand_Dev_AllowsRm(t *testing.T) {
	if err := ValidateCommand("dev", "rm -rf ./tmp", nil); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
}

func TestValidateCommand_Coding_AllowsCommonDevCommands(t *testing.T) {
	cases := []string{
		"git status",
		"git diff --name-only",
		"find . -name \"*.go\"",
		"go test ./...",
		"gofmt -w .",
		"python -V",
		"python3 -V",
		"pip -V",
		"pytest -q",
		"node -v",
		"npm test",
		"pnpm -v",
		"yarn -v",
		"make test",
	}
	for _, cmd := range cases {
		if err := ValidateCommand("coding", cmd, nil); err != nil {
			t.Fatalf("expected allow for %q, got %v", cmd, err)
		}
	}
}

func TestValidateCommand_Coding_AllowsRm(t *testing.T) {
	if err := ValidateCommand("coding", "rm -rf ./tmp", nil); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
}

func TestValidateCommand_Coding_AllowsFdRedirectionToStdout(t *testing.T) {
	cases := []string{
		"python3 -V 2>&1",
		"echo hi 2>&1",
		"python3 -m py_compile main.py 2>&1",
		"echo A && python3 -V 2>&1",
	}
	for _, cmd := range cases {
		if err := ValidateCommand("coding", cmd, nil); err != nil {
			t.Fatalf("expected allow for %q, got %v", cmd, err)
		}
	}
}

func TestValidateCommand_Coding_DeniesSudo(t *testing.T) {
	if err := ValidateCommand("coding", "sudo ls", nil); err == nil {
		t.Fatalf("expected deny for sudo")
	}
}

func TestExtractCommands_SplitsCommandSegments(t *testing.T) {
	got := ExtractCommands(`A=1 echo hi 2>&1 && rm -rf a | grep x; python -V`)
	want := []string{"echo", "rm", "grep", "python"}
	if len(got) != len(want) {
		t.Fatalf("unexpected tokens: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected tokens: got=%v want=%v", got, want)
		}
	}
}
