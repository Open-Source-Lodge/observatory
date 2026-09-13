# Development

## Demo GIF

The `README.md` shows `assets/demo.gif`. Make the GIF again when the
interface changes:

```sh
brew install asciinema agg font-dejavu
sh assets/demo.sh
```

The script builds observatory, makes a small repository with two rules and a
commit that breaks one of them in a temporary directory, starts
`assets/demo_api.py` as a local replacement for the API, records a session with
`assets/demo.exp`, and writes the GIF.

## Release

The `release` workflow runs [release-please](https://github.com/googleapis/release-please)
each time a pull request merges to `main`. Release-please reads the commit
messages since the last release. Write the commit messages in the Conventional
Commits format. Release-please selects the version number with these rules:

| commit message                                | version change |
| --------------------------------------------- | -------------- |
| `feat!:` or a `BREAKING CHANGE:` footer       | major          |
| `feat:`                                       | minor          |
| `fix:` or `perf:`                             | patch          |
| other types, such as `docs:` or `ci:`         | no release     |

Release-please opens a release pull request. The pull request updates
`CHANGELOG.md` with the new version. Merge the release pull request to make
the tag, such as `v1.2.3`, and the GitHub release.

When you merge the release pull request, the `binaries` job builds `observatory`
for macOS, Linux and Windows, on the amd64 and arm64 architectures. The job
attaches the binaries and `checksums.txt` to the GitHub release. The job also
sets the version in the binaries, so that `observatory version` shows the tag.
Then the job moves the major tag, such as `v1`, to the release, so that
`uses: Open-Source-Lodge/observatory@v1` installs the newest release.

## Dependabot

Dependabot opens one pull request each week for the Go modules and one for the
actions. The commits use the `chore:` type. A `chore:` or `deps:` commit makes
no release.
