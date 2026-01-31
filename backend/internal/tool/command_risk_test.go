package tool

import "testing"

func TestIsHighRiskCommand(t *testing.T) {
	cases := []struct {
		name    string
		command string
		want    bool
	}{
		{name: "safe_ls", command: "ls -la", want: false},
		{name: "safe_git_status", command: "git status", want: false},
		{name: "git_clean_dry_run", command: "git clean -n", want: false},
		{name: "git_clean_force", command: "git clean -fd", want: true},
		{name: "git_reset_hard", command: "git reset --hard HEAD~1", want: true},
		{name: "rm", command: "rm -rf a", want: true},
		{name: "mv", command: "mv a b", want: true},
		{name: "find_delete", command: "find . -name '*.tmp' -delete", want: true},
		{name: "find_exec", command: "find . -type f -exec rm {} \\;", want: true},
		{name: "npm_install", command: "echo hi && npm install", want: true},
		{name: "python_exec", command: "A=1 echo hi; python -V", want: true},
		{name: "run_shell", command: "bash -lc 'echo hi'", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isHighRiskCommand(tc.command); got != tc.want {
				t.Fatalf("isHighRiskCommand(%q)=%v want=%v", tc.command, got, tc.want)
			}
		})
	}
}
