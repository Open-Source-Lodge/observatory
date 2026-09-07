package main

import (
	"errors"
	"strings"
	"testing"
)

// stubGit replaces command with a function that answers from a map of the
// joined git arguments to the output.
func stubGit(t *testing.T, answers map[string]string) *[]string {
	t.Helper()
	var calls []string
	old := command
	command = func(name string, args ...string) (string, error) {
		key := strings.Join(args, " ")
		calls = append(calls, key)
		out, ok := answers[key]
		if !ok {
			return "", errors.New("stub: no answer for " + key)
		}
		return out, nil
	}
	t.Cleanup(func() { command = old })
	return &calls
}

func TestChangesDefaultIsLastCommit(t *testing.T) {
	t.Setenv("GITHUB_BASE_REF", "")
	stubGit(t, map[string]string{
		"show --format=commit %H%n%s%n%n%b --patch HEAD": "commit abc\n+x",
	})
	s := Scope{}
	got, err := changes(&s)
	if err != nil {
		t.Fatal(err)
	}
	if got != "commit abc\n+x" || s.Commit != "HEAD" {
		t.Errorf("got %q with scope %+v", got, s)
	}
}

func TestChangesBase(t *testing.T) {
	calls := stubGit(t, map[string]string{
		"merge-base origin/main HEAD": "abc123",
		"diff abc123":                 "+the diff",
	})
	s := Scope{Base: "origin/main"}
	got, err := changes(&s)
	if err != nil || got != "+the diff" {
		t.Errorf("got %q, %v", got, err)
	}
	if len(*calls) != 2 {
		t.Errorf("calls: %v", *calls)
	}
}

func TestChangesUncommitted(t *testing.T) {
	stubGit(t, map[string]string{"diff HEAD": "+wip"})
	s := Scope{Uncommitted: true}
	if got, err := changes(&s); err != nil || got != "+wip" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestChangesGitHubBaseRef(t *testing.T) {
	t.Setenv("GITHUB_BASE_REF", "develop")
	stubGit(t, map[string]string{
		"merge-base origin/develop HEAD": "abc123",
		"diff abc123":                    "+the diff",
	})
	s := Scope{}
	if _, err := changes(&s); err != nil {
		t.Fatal(err)
	}
	if s.Base != "origin/develop" {
		t.Errorf("base = %q", s.Base)
	}
}

func TestChangesCommit(t *testing.T) {
	stubGit(t, map[string]string{
		"show --format=commit %H%n%s%n%n%b --patch deadbeef": "commit deadbeef\n+x",
	})
	s := Scope{Commit: "deadbeef"}
	got, err := changes(&s)
	if err != nil || !strings.HasPrefix(got, "commit deadbeef") {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestIgnored(t *testing.T) {
	tests := []struct {
		pattern, file string
		want          bool
	}{
		{"main.py", "main.py", true},
		{"main.py", "src/main.py", true},
		{"src/main.py", "src/main.py", true},
		{"src/main.py", "lib/src/main.py", false},
		{"vendor/", "vendor/a/b.go", true},
		{"vendor", "a/vendor/b.go", true},
		{"vendor", "vendors/b.go", false},
		{"*.md", "docs/a/b.md", true},
		{"docs/*", "docs/a/b.md", true},
		{"docs/*.md", "docs/a/b.md", false},
		{"**/*_test.go", "a/b/c_test.go", true},
		{"**/testdata", "a/testdata/x", true},
	}
	for _, tt := range tests {
		if got := ignored([]string{tt.pattern}, tt.file); got != tt.want {
			t.Errorf("ignored(%q, %q) = %v, want %v", tt.pattern, tt.file, got, tt.want)
		}
	}
}

func TestWithoutIgnored(t *testing.T) {
	diff := "commit abc\nsubject\n\ndiff --git a/main.py b/main.py\n+import requests\ndiff --git a/docs/a.md b/docs/a.md\n+# hi\n"
	got := withoutIgnored(diff, []string{"*.md"})
	if got != "commit abc\nsubject\n\ndiff --git a/main.py b/main.py\n+import requests\n" {
		t.Errorf("got %q", got)
	}
	if got := withoutIgnored(diff, nil); got != diff {
		t.Errorf("no patterns changed the diff: %q", got)
	}
	if got := withoutIgnored(diff, []string{"*.md", "main.py"}); got != "" {
		t.Errorf("every file ignored, got %q", got)
	}
	files := "==== a/b.go ====\npackage a\n==== c.md ====\n# c\n"
	if got := withoutIgnored(files, []string{"a/"}); got != "==== c.md ====\n# c\n" {
		t.Errorf("all files: got %q", got)
	}
	if got := withoutIgnored("diff --git x x\n+1\n", []string{"x"}); got != "" {
		t.Errorf("noprefix: got %q", got)
	}
}
