# Schema changes come with a migration

A change to a model under `models/` that adds, removes or renames a column or a table comes with a new migration file under `migrations/` in the same diff. A migration does not change a migration that is already merged.
