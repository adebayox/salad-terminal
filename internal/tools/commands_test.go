package tools

import "testing"

func TestParseCommand(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want []string
		err  bool
	}{
		{name: "simple", cmd: "go test ./...", want: []string{"go", "test", "./..."}},
		{name: "quoted arg", cmd: `echo "hello world"`, want: []string{"echo", "hello world"}},
		{name: "single quotes", cmd: `git commit -m 'fix bug'`, want: []string{"git", "commit", "-m", "fix bug"}},
		{name: "escaped space", cmd: `cat my\ file.txt`, want: []string{"cat", "my file.txt"}},
		{name: "pipe rejected", cmd: "ls | wc", err: true},
		{name: "semicolon rejected", cmd: "ls; rm -rf /", err: true},
		{name: "redirect rejected", cmd: "echo hi > out.txt", err: true},
		{name: "backtick rejected", cmd: "echo `ls`", err: true},
		{name: "command substitution rejected", cmd: "echo $(whoami)", err: true},
		{name: "unterminated quote", cmd: `echo "oops`, err: true},
		{name: "empty", cmd: "   ", err: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseCommand(tc.cmd)
			if tc.err {
				if err == nil {
					t.Fatalf("expected error for %q, got %v", tc.cmd, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want Classification
	}{
		{name: "git status auto", argv: []string{"git", "status", "--short"}, want: ClassificationAuto},
		{name: "git diff auto", argv: []string{"git", "diff", "--stat"}, want: ClassificationAuto},
		{name: "git log auto", argv: []string{"git", "log", "-5"}, want: ClassificationAuto},
		{name: "git checkout approval", argv: []string{"git", "checkout", "main"}, want: ClassificationApproval},
		{name: "git reset approval", argv: []string{"git", "reset", "--hard"}, want: ClassificationApproval},
		{name: "git branch -d approval", argv: []string{"git", "branch", "-d", "old"}, want: ClassificationApproval},
		{name: "ls auto", argv: []string{"ls", "-la"}, want: ClassificationAuto},
		{name: "pwd auto", argv: []string{"pwd"}, want: ClassificationAuto},
		{name: "go version auto", argv: []string{"go", "version"}, want: ClassificationAuto},
		{name: "go env auto", argv: []string{"go", "env"}, want: ClassificationAuto},
		{name: "go test approval", argv: []string{"go", "test", "./..."}, want: ClassificationApproval},
		{name: "go build approval", argv: []string{"go", "build", "./..."}, want: ClassificationApproval},
		{name: "npm test approval", argv: []string{"npm", "test"}, want: ClassificationApproval},
		{name: "unknown approval", argv: []string{"golangci-lint", "run"}, want: ClassificationApproval},
		{name: "empty denied", argv: nil, want: ClassificationDenied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := Classify(tc.argv)
			if got != tc.want {
				t.Fatalf("Classify(%v) = %d, want %d", tc.argv, got, tc.want)
			}
		})
	}
}
