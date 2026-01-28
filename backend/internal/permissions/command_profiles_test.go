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
