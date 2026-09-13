package observatory

import (
	"errors"
	"slices"
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
	s.resolve()
	got, err := changes(s, nil)
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
	got, err := changes(s, nil)
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
	if got, err := changes(s, nil); err != nil || got != "+wip" {
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
	s.resolve()
	if _, err := changes(s, nil); err != nil {
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
	got, err := changes(s, nil)
	if err != nil || !strings.HasPrefix(got, "commit deadbeef") {
		t.Errorf("got %q, %v", got, err)
	}
}

// TestPathspecs checks the shape of the pathspecs. git itself decides which
// file a pathspec matches; `git ls-files -- <pathspec>` shows that.
func TestPathspecs(t *testing.T) {
	if got := pathspecs(nil); got != nil {
		t.Errorf("no patterns gave %v", got)
	}
	tests := []struct{ pattern, want string }{
		// A pattern without a slash matches at every depth.
		{"main.py", "**/main.py"},
		// A pattern with a slash starts at the root of the repository.
		{"src/main.py", "src/main.py"},
		// A directory loses its slash and keeps the depth.
		{"vendor/", "**/vendor"},
		{"docs/*", "docs/*"},
		{"**/*_test.go", "**/*_test.go"},
	}
	for _, tt := range tests {
		got := pathspecs([]string{tt.pattern})
		want := []string{"--", ":(exclude,top,glob)" + tt.want, ":(exclude,top,glob)" + tt.want + "/**"}
		if !slices.Equal(got, want) {
			t.Errorf("pathspecs(%q) = %v, want %v", tt.pattern, got, want)
		}
	}
	if got := pathspecs([]string{"a", "b"}); len(got) != 5 || got[0] != "--" {
		t.Errorf("two patterns gave %v", got)
	}
}

// TestChangesIgnore checks that the patterns reach git as pathspecs.
func TestChangesIgnore(t *testing.T) {
	calls := stubGit(t, map[string]string{
		"diff HEAD -- :(exclude,top,glob)**/*.md :(exclude,top,glob)**/*.md/**": "+code",
	})
	got, err := changes(Scope{Uncommitted: true}, []string{"*.md"})
	if err != nil || got != "+code" {
		t.Fatalf("got %q, %v", got, err)
	}
	if len(*calls) != 1 {
		t.Errorf("calls: %v", *calls)
	}
}
