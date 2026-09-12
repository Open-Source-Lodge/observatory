package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func git(args ...string) (string, error) {
	return command("git", args...)
}

// command is a variable, so that a test can replace the real process.
var command = func(name string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Scope says which part of the repo a run checks. At most one field has a
// value.
// The zero Scope is the last commit, or the pull request in GitHub Actions.
type Scope struct {
	Base        string
	Commit      string
	All         bool
	Uncommitted bool
}

// String describes the scope for the prompt and for the report.
func (s Scope) String() string {
	switch {
	case s.All:
		return "every tracked file in the repository"
	case s.Uncommitted:
		return "the uncommitted changes"
	case s.Base != "":
		return "the changes since " + s.Base
	default:
		return "the commit " + s.Commit
	}
}

// parseScope reads the scope flags of `run`.
func parseScope(args []string) (Scope, error) {
	var s Scope
	for i := 0; i < len(args); i++ {
		arg, val, hasVal := strings.Cut(args[i], "=")
		next := func() (string, error) {
			if hasVal {
				return val, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s needs a ref", arg)
			}
			i++
			return args[i], nil
		}
		var err error
		switch arg {
		case "--base":
			s.Base, err = next()
		case "--commit":
			s.Commit, err = next()
		case "--all":
			s.All = true
		case "--uncommitted":
			s.Uncommitted = true
		default:
			return s, fmt.Errorf("unknown argument %q", args[i])
		}
		if err != nil {
			return s, err
		}
	}
	set := 0
	for _, on := range []bool{s.Base != "", s.Commit != "", s.All, s.Uncommitted} {
		if on {
			set++
		}
	}
	if set > 1 {
		return s, errors.New("use only one of --base, --commit, --uncommitted and --all")
	}
	return s, nil
}

// resolve fills the empty scope: the pull request in GitHub Actions, or the
// last commit.
func (s *Scope) resolve() {
	if *s != (Scope{}) {
		return
	}
	if ref := os.Getenv("GITHUB_BASE_REF"); ref != "" {
		s.Base = "origin/" + ref
	} else {
		s.Commit = "HEAD"
	}
}

// changes collects the text the model reads. The text is a diff, or every
// tracked file for --all. git leaves out the files that the ignore patterns
// of the rule match.
func changes(s Scope, ignore []string) (string, error) {
	paths := pathspecs(ignore)
	switch {
	case s.All:
		return allFiles(paths)
	case s.Uncommitted:
		return git(append([]string{"diff", "HEAD"}, paths...)...)
	case s.Commit != "":
		return git(append([]string{"show", "--format=commit %H%n%s%n%n%b", "--patch", s.Commit}, paths...)...)
	}
	// The diff is from the fork point, so commits already on the base branch
	// do not count. It goes up to the working tree, so an uncommitted change
	// counts too.
	fork, err := git("merge-base", s.Base, "HEAD")
	if err != nil {
		return "", fmt.Errorf("cannot find where %s and HEAD diverge: %w", s.Base, err)
	}
	return git(append([]string{"diff", fork}, paths...)...)
}

// pathspecs turns the ignore patterns of a rule into git pathspecs that
// exclude the files, with the `--` separator in front. The result is empty
// when there are no patterns.
//
// A pattern is a file, a directory, or a glob. A pattern with a slash starts
// at the root of the repository. A pattern without a slash, or with the
// prefix `**/`, matches at every depth. Each pattern gives two pathspecs: one
// for the path, and one for the files below it, because a directory does not
// exclude its contents by itself.
func pathspecs(ignore []string) []string {
	if len(ignore) == 0 {
		return nil
	}
	out := []string{"--"}
	for _, p := range ignore {
		p = strings.TrimSuffix(p, "/")
		if !strings.Contains(p, "/") {
			p = "**/" + p
		}
		out = append(out, ":(exclude,top,glob)"+p, ":(exclude,top,glob)"+p+"/**")
	}
	return out
}

// allFiles renders every tracked text file with a header, like a diff would.
func allFiles(paths []string) (string, error) {
	root, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	out, err := git(append([]string{"ls-files"}, paths...)...)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, path := range strings.Split(out, "\n") {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			continue // unreadable or binary
		}
		fmt.Fprintf(&b, "==== %s ====\n%s\n", path, data)
	}
	return b.String(), nil
}
