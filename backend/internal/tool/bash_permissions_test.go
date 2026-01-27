package tool

import "testing"

func TestBashCommandContainsToken_Rm(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"rm -rf /tmp/a", true},
		{"sudo rm -rf /tmp/a", true},
		{"echo hi; rm -rf /tmp/a", true},
		{"echo hi && rm -rf /tmp/a", true},
		{"echo hi | rm -rf /tmp/a", true},
		{"rmdir /tmp/a", false},
		{"echo arm", false},
		{"echo hi", false},
	}

	for _, tc := range cases {
		got := bashCommandContainsToken(tc.cmd, "rm")
		if got != tc.want {
			t.Fatalf("cmd=%q want=%v got=%v", tc.cmd, tc.want, got)
		}
	}
}

