# Schema changes come with a migration

## Why

The database in production does not read the model files. It reads the
migrations. A model change without a migration works on a fresh development
database and fails in production.

A merged migration has already run in production. A change to it does not
run again there, so the two databases drift apart.

## What the rule covers

- Each new, removed or renamed field on a model under `models/`.
- Each change to the type, the default or the constraints of a field.
- Each change to a file under `migrations/` that git already tracks.

## What the rule does not cover

- A change to a method on a model.
- A change to a field that does not reach the database, such as a property.
