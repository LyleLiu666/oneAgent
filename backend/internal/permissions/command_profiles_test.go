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

