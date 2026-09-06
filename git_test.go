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
