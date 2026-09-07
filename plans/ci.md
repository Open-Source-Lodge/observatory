# Plan: dependency and toolchain checks

## Goal

CI finds a known vulnerability, a dirty `go.sum`, or an old dependency before
a user does. A user with `go install` does not download a Go toolchain that
they do not need.

## Current state

The `ci` workflow runs gofmt, `go vet`, staticcheck and the tests. Nothing
checks the dependencies for vulnerabilities or for new versions. `go.mod`
says `go 1.26.5`. A patch version in the `go` line makes each `go install`
download that exact toolchain.

## Steps

1. `go.mod`: change the `go` line to `go 1.26`. Add `toolchain go1.26.5` if CI must run on that patch version.
2. `lint` job: add `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` after the staticcheck step. Add the same line to the `lint` target in the `Makefile`.
3. `lint` job: add a step that runs `go mod tidy` and then `git diff --exit-code go.mod go.sum`.
4. Add `.github/dependabot.yml` with two entries: `gomod` and `github-actions`, both with a weekly schedule.
5. `DEVELOPMENT.md`: one paragraph on the dependabot pull requests. A `deps:` or `chore:` commit makes no release.

## Open questions

- Dependabot pull requests for the Charm libraries can come each week. Group them in one pull request with the `groups` key, or accept the noise.
- Pin the `govulncheck` version, or follow `@latest` as staticcheck does today.
