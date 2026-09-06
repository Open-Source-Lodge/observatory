# Keep the public API backward compatible

A change to a file under `api/` does not remove an endpoint, remove a field from a response, rename a field, or make an optional parameter required. Add a new endpoint or a new version under `api/v2/` instead. A change with `BREAKING CHANGE:` in the commit message is exempt.
