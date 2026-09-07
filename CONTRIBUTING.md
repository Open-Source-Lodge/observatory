# Contributing

## Build and test

```sh
make            # lint, test, build
make test
make lint
```

## Commit messages

Write the commit messages in the Conventional Commits format. The release
tool reads the commit types to select the version number. `DEVELOPMENT.md`
has the table of the version changes.

## Documentation

Write all documentation in ASD-STE100 Simplified Technical English. The
"Documentation rules" section of `README.md` has the rules.

## Pull requests

The repository checks its own pull requests with observatory. The rules are in
`.observatory/`, and the workflow is `.github/workflows/observatory.yml`.
