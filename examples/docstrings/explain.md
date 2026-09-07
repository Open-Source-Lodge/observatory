# Public functions have a docstring

## Why

The docstring is the contract of the function. The editor shows it, the API
documentation comes from it, and a reviewer reads it before the code.

## What the rule covers

- Each new or changed function, method and class with a public name.
- The first line of the docstring: one sentence, in the present tense.

## What the rule does not cover

- Names that start with `_`.
- Test functions and test classes.
- Trivial overrides, such as `__str__`, when the parent has a docstring.
