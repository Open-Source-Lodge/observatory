# Code changes come with tests

## Why

A test that lands in the same change as the code shows what the change does
and keeps it that way. A test that comes later usually does not come.

## What the rule covers

- Each diff that adds or changes logic under `src/`.
- The diff must also add or change a file under `tests/`.

## What the rule does not cover

- A rename with no change to the logic.
- A change to comments, docstrings or documentation.
- A change to configuration files.

## How the model judges this rule

The model sees the whole diff at one time. Thus it can see that `src/` and
`tests/` both change. It cannot see whether the test covers the change well.
A human reviewer does that.
