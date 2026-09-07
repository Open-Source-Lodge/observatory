# Plan: repository files for contributors and users

## Goal

A contributor finds the commit rules and the security contact without a
search. The `pkg.go.dev` page shows a description. The module that `go
install` downloads contains only what a user needs.

## Current state

`DEVELOPMENT.md` has the release and the commit message rules, but GitHub
shows no "Contributing" or "Security policy" link. `main.go` has no package
comment, so `pkg.go.dev` shows an empty page. The `plans/` directory ships
with the module.

## Steps

1. `main.go`: add a package comment above `package main`. One or two sentences that say what observatory does.
2. Add `SECURITY.md`: how to report a vulnerability, and where. GitHub shows it in the "Security" tab.
3. Add `CONTRIBUTING.md`: the Conventional Commits rules from `DEVELOPMENT.md`, the `make` targets, and the documentation rules from the README. Link to the two files in place of a copy where the text is long.
4. Add `.gitattributes` with `plans/ export-ignore`, so that the module zip has no plans.

## Open questions

- Keep `DEVELOPMENT.md` next to `CONTRIBUTING.md`, or move its content into `CONTRIBUTING.md` and delete it. One file is simpler.
- Report a vulnerability by email, or with the GitHub private vulnerability report. The GitHub report needs no address in the repository.
