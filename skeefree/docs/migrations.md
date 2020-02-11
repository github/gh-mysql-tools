# Migration strategies and states

## Strategies

A migration has one of the following strategies:

- `manual`: `skeefree` will not run this migration.
- `direct`: `skeefree` will issue the migration statement directly on the cluster's `master`.
  This is the strategy used for `CREATE` and `DROP` migrations. Note that `DROP` migrations are actually executed as `RENAME TABLE`.
- `gh-ost`: `skeefree` will run the migration through `gh-ost`. `skeefree` does not directly execute `gh-ost`. `skeefree` is a (internal to GitHub) kubernetes app, and`gh-ost` migrations can run for days; the two don't work well together. Instead, utility boxes use `skeefree` (CLI mode) to determine whether they should spawn a `gh-ost` migration.

It is possible to `.skeefree approve-autorun` a migration with a `manual` strategy. Depending on the migration statement, this will change the strategy to either `direct` or `gh-ost`.

## States

A migration goes through a number of steps, from first being detected until fully applied. The following elaborates on the states and how they transition.

- `proposed`: `skeefree` has just detected a PR. It analyzed the output from `skeema-diff`, and based on information on the affected cluster (is it `vitess`? It is sharded?) creates one or more `proposed` migrations.
  databases team has not approved the PR yet.
- `queued`: when the PR has been approved by databases team. From this moment on, automation takes over the migration. A `queued` migration awaits being scheduled by `skeefree`. Potentially it will be next up within a moment; or it may wait on a another, long-running migration, executing on same cluster. A migration might spend time in `queued` for a minute to multiple days.
- `ready`: `skeefree` has decided that a migration can start running. From this moment on the migration is imminent. Executors (e.g. utility hosts) will compete to run the migration.
- `running`: the migration has started. More so, it's been lively in the past few minutes.
  For `direct` migrations, this `state` is very brief. For `gh-ost` migrations, this may take seconds, minutes, hours or days. During runtime, `gh-ost` hooks keep pushing life signals to `skeefree`.
- `complete`: when the migration is successful and the schema change has been applied. In the case of `gh-ost` this means the cut-over has been performed.
- `failed`: when the migration is unsuccessful. Example: someone issued a `.migration panic <table_name>`. See notes below.
- `cancelled`: if someone cancelled the migration. See notes below.

### Notes

- A human may retry a `failed` migration via `.skeefree retry-migration <pr url> <table_name>`. This will change migration's state to `queued`, to be scheduled again by `skeefree`. Note that for `gh-ost` migrations, you might want to see if a `_gho` table still exists, or else the new migration may fail.
- For migrations with `manual` strategy, a human can run the migration manually, then issue a `.skeefree mark-complete <pr url> <table_name>`. This transitions the migration's state to `complete`.
- A PR can be _cancelled_. This is assuming none of its migrations have executed. A `.skeefree cancel-pr <pr url>` transitions migrations of this PR to `cancelled` state. There's no mechanism to un-cancel a migration.
