# Keep the public API backward compatible

## Why

Other teams and customers call this API. We do not control when they update
their code. A removed field breaks their code at a time we do not choose.

## What the rule covers

- Each removed endpoint, route or handler under `api/`.
- Each removed or renamed field in a response type.
- Each optional parameter that becomes required.
- Each change to the type of a field, such as a string that becomes a number.

## What the rule does not cover

- A new endpoint, a new optional parameter or a new field in a response.
- A change under `api/v2/` before the release of that version.
- A change that the commit message marks with `BREAKING CHANGE:`. That
  change goes through the deprecation process in `docs/api-policy.md`.
