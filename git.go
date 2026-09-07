package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
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

// changes collects the text the model reads, and resolves the zero scope.
// The text is a diff, or every tracked file for --all.
func changes(s *Scope) (string, error) {
	if *s == (Scope{}) {
		if ref := os.Getenv("GITHUB_BASE_REF"); ref != "" {
			s.Base = "origin/" + ref
		} else {
			s.Commit = "HEAD"
		}
	}
	switch {
	case s.All:
		return allFiles()
	case s.Uncommitted:
		return git("diff", "HEAD")
	case s.Commit != "":
		return git("show", "--format=commit %H%n%s%n%n%b", "--patch", s.Commit)
	}
	// The diff is from the fork point, so commits already on the base branch
	// do not count. It goes up to the working tree, so an uncommitted change
	// counts too.
	fork, err := git("merge-base", s.Base, "HEAD")
	if err != nil {
		return "", fmt.Errorf("cannot find where %s and HEAD diverge: %w", s.Base, err)
	}
	return git("diff", fork)
}

// allFiles renders every tracked text file with a header, like a diff would.
func allFiles() (string, error) {
	root, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	out, err := git("ls-files")
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

// withoutIgnored removes the files that match a pattern from the changes.
// A change is a git diff, or the output of allFiles. The text before the
// first file, such as the commit message, stays. The result is empty when
// no file remains.
func withoutIgnored(diff string, patterns []string) string {
	if len(patterns) == 0 {
		return diff
	}
	var b strings.Builder
	keep, kept := true, false
	for _, line := range strings.SplitAfter(diff, "\n") {
		if file, ok := changedFile(line); ok {
			keep = !ignored(patterns, file)
			kept = kept || keep
		}
		if keep {
			b.WriteString(line)
		}
	}
	if !kept {
		return ""
	}
	return b.String()
}

// changedFile is the path in the header line of one file: `diff --git a/x b/x`
// from git, or `==== x ====` from allFiles.
func changedFile(line string) (string, bool) {
	line = strings.TrimRight(line, "\n")
	if rest, ok := strings.CutPrefix(line, "==== "); ok {
		return strings.TrimSuffix(rest, " ===="), true
	}
	rest, ok := strings.CutPrefix(line, "diff --git ")
	if !ok {
		return "", false
	}
	if _, file, found := strings.Cut(rest, " b/"); found {
		return file, true
	}
	// ponytail: git with diff.noprefix shows `diff --git x x`.
	return rest[strings.LastIndexByte(rest, ' ')+1:], true
}

// ignored reports whether a file matches one of the patterns. A pattern is a
// file, a directory, or a glob in the syntax of path.Match. A pattern with a
// slash starts at the root of the repository. A pattern without a slash, or
// with the prefix `**/`, matches at every depth. A directory matches every
// file in it.
func ignored(patterns []string, file string) bool {
	segs := strings.Split(file, "/")
	for _, p := range patterns {
		p = strings.TrimSuffix(p, "/")
		anywhere := !strings.Contains(p, "/")
		if rest, ok := strings.CutPrefix(p, "**/"); ok {
			p, anywhere = rest, true
		}
		for start := 0; start < len(segs) && (anywhere || start == 0); start++ {
			for end := start + 1; end <= len(segs); end++ {
				if ok, _ := path.Match(p, strings.Join(segs[start:end], "/")); ok {
					return true
				}
			}
		}
	}
	return false
}
