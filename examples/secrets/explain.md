# No secrets in the repository

## Why

A secret in git stays in the history after you remove it from the file. Each
clone of the repository then holds the secret. The only fix is to rotate the
secret.

## What the rule covers

- Each string that looks like a key, a token or a password, in each file type.
- Each URL with a user name and a password in it.
- Each private key block, such as `-----BEGIN PRIVATE KEY-----`.

## What the rule does not cover

- Example values in documentation that are clearly not real.
- Names of environment variables, such as `ANTHROPIC_API_KEY`.

## Note

This rule is the second check. Use a pre-commit hook or a secret scanner
as the first check, because those tools run before the commit.
