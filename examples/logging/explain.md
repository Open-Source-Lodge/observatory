# No print statements

## Why

A `print` call writes to stdout and has no level, no timestamp and no module
name. The logger has all three, and an operator can change the level of a module
without a change to the code.

## What the rule covers

- Each `print(` call in the source, outside `cli/`.
- Each `sys.stdout.write` and `sys.stderr.write` call in the source.

## What the rule does not cover

- Entry points under `cli/`, because the user reads that output.
- Test code.
