# Examples

This directory holds example rules. Each subdirectory is one rule, with the
two files that observatory reads: `rule.md` and `explain.md`. The name of
the subdirectory says what type of rule it is.

To use an example, copy its directory into `.observatory` and give it an
ID:

```sh
cp -r examples/logging .observatory/OBS-003
```

Then change the text so that it matches your repository.

| example               | type of rule                                    |
| --------------------- | ----------------------------------------------- |
| `library-choice`      | one library for one task                        |
| `logging`             | no direct output; use the logger                |
| `docstrings`          | each public function has documentation          |
| `tests`               | a change to the code comes with a test          |
| `secrets`             | no credentials in the repository                |
| `error-handling`      | errors are not ignored                          |
| `database-migrations` | a schema change comes with a migration          |
| `public-api`          | a change to the public API is backward compatible |
| `rate-limiting`       | each call to an external service has a rate limit |
| `documentation-style` | documentation follows ASD-STE100                  |
