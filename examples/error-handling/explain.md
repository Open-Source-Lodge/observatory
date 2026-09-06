# Do not ignore errors

## Why

An error that a function ignores comes back later, in a different place,
with less context. The line that ignored it is the cheapest place to fix it.

## What the rule covers

- In Go: `_ = f()` where `f` returns an error, and `v, _ := f()`.
- In Python: `except Exception: pass`, and a bare `except:`.
- In JavaScript: `.catch(() => {})` and an empty `catch` block.

## What the rule does not cover

- A handler with a comment that says why the error is safe to ignore, such
  as `// the file is already gone; that is the goal`.
- A `defer f.Close()` on a file that the code only reads.
