# Plan: release system

## Goal

The same flow as forestry: release-please reads the commit messages, opens a
release pull request, and a merge of that pull request makes the tag, the
GitHub release, and the binaries.

## Steps

1. First commit as `feat: first pass of observatory`, so that release-please starts at `v0.1.0`. Push to `main` on GitHub under `Open-Source-Lodge`.
2. Repository settings: Actions, "Workflow permissions", set "Read and write" and "Allow GitHub Actions to create and approve pull requests". Release-please needs both.
3. Merge the first release pull request. Check the tag, the release, and the four binaries.
4. `action.yml`: replace `go install` with a download of the release binary for the runner, so the action does not need a Go toolchain. Keep `version: latest` as the default and read the tag from the GitHub API.
5. Add a step to the `binaries` job that moves the major tag (`v1`) to the new release, so that `uses: Open-Source-Lodge/observatory@v1` works.
6. README: an "Install" line for the release binaries.

## Open questions

- Homebrew tap or a `curl | sh` installer: skip until somebody asks.
- Should the action pin its own version in a test workflow of this repository, to dogfood `OBS-001` on each pull request? Cheap to add, and it needs the API key as a repository secret.
