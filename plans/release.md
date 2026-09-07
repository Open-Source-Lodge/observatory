# Plan: release system

## Goal

The same flow as forestry: release-please reads the commit messages, opens a
release pull request, and a merge of that pull request makes the tag, the
GitHub release, and the binaries. A user installs a release binary without a
Go toolchain, and can verify the download.

## Current state

The `release` workflow runs release-please and builds four binaries. The
release has no checksums and no Windows binary. The action runs `go install`
on each run, so it needs a Go toolchain and compiles the module each time. The
README tells users to use `Open-Source-Lodge/observatory@main`.

## Steps

1. First commit as `feat: first pass of observatory`, so that release-please starts at `v0.1.0`. Push to `main` on GitHub under `Open-Source-Lodge`.
2. Repository settings: Actions, "Workflow permissions", set "Read and write" and "Allow GitHub Actions to create and approve pull requests". Release-please needs both.
3. Merge the first release pull request. Check the tag, the release, and the four binaries.
4. `binaries` job: add `windows/amd64` and `windows/arm64` to the targets, with the `.exe` suffix. Or write in the README that observatory does not support Windows.
5. `binaries` job: write `checksums.txt` with `sha256sum observatory-*` and upload it with the binaries.
6. `action.yml`: replace `go install` with a download of the release binary for the runner, so the action does not need a Go toolchain. Check the download against `checksums.txt`. Keep `version: latest` as the default and read the tag from the GitHub API.
7. Add a step to the `binaries` job that moves the major tag (`v1`) to the new release, so that `uses: Open-Source-Lodge/observatory@v1` works.
8. README: an "Install" line for the release binaries, and `@v1` in place of `@main` in the action examples.

## Open questions

- Homebrew tap or a `curl | sh` installer: skip until somebody asks.
- GoReleaser: skip. The loop in the `binaries` job does the same work for six targets and one checksum file.
- Should the action pin its own version in a test workflow of this repository, to check `OBS-001` on each pull request of this repository? Cheap to add, and it needs the API key as a repository secret.
