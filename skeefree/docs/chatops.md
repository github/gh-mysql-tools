# Chatops

`.skeefree` chatops are listed below by category


### Status

- `.skeefree sup`
  Oriented towards schema-reviewers team usage, this shows PRs in various review stages.
  For each PR we list the migrations in the PR and their state.

- `.skeefree status`
  Oriented towards database team usage, this shows migrations by state: completed, running, ready, queued, etc.
  Towards the top you will find most recent and most "interesting" migrations, like those recently completed or those which are running.

See [migration strategies and states](migrations.md) to better understand the output.

### Repositories

- Adding a new repository:
  `.skeefree add-repo <[myorg/]repo> <team>`
  example: `.skeefree add-repo myorg/myorg my-team`

  This makes `skeefree` know about a repository. `skeefree` will henceforth start polling this repository looking for migration PRs.
  But this setup is incomplete, see next steps.

- Map repository schema to production target. A repository will have one or more `skeema`-defined schemas.
  `.skeefree repo-map <[myorg/]repo> <hint> <cluster> <schema>`
  example: `.skeefree repo-map myorg/skeefree-test skeefree_test testing-cluster skeefree_test`

  This should typically take place immediately after `.skeefree add-repo` and as part of the initial setup.
  This tells `skeefree` where to deploy schema changes to.
  `<hint>` can be the schema name as fiend in the `.skeema` file in the repo. If there's multiple `.skeema` files, you will use one `.skeefree repo-map` per `.skeema` file.

  You may override a previous map by remapping with same hint, e.g.:
  `.skeefree repo-map myorg/skeefree-test skeefree_test bagpipes some_schema`

- Remove a mapping:
  `.skeefree repo-unmap <[myorg/]repo> <hint>`
  example: `.skeefree repo-unmap myorg/skeefree-test skeefree_test`

- Enable/disable automatic migrations for a repo:
  `.skeefree repo-autorun <enable|disable> <[myorg/]repo>`
  example: `.skeefree repo-autorun enable myorg/skeefree-test`

- Updating a repository's owner team:
  `.skeefree update-repo <[myorg/]repo> <team>`
  example: `.skeefree add-repo myorg/launch some-other-team`

- Removing a repository:
  `.skeefree remove-repo <[myorg/]repo> <team>`
  example: `.skeefree remove-repo myorg/bagpipes`

  `skeefree` will no longer poll this repository for migration PRs.

- Show a repository:
  `.skeefree show-repo <[myorg/]repo>`
  example: `.skeefree show-repo myorg/my_repo`

  shows repo, owner, if it's enabled to auto-run, mappings.

- Show all repositories in detail:
  `.skeefree show-repos`

  for all known repos, show repo name, owner, if it's enabled to auto-run, mappings.

- List known repos:
  `.skeefree which-repos`

  show known repo names.

### Pull requests

`skeefree` automatically probes the known repos for pull requests with migrations.

- Show details for a pull request:
  `.skeefree show-pr <pr url>`
  Provide the full URL for a PR.
  example: `.skeefree show-pr https://github.com/myorg/my_repo/pull/54`

  Output indicates state of the PR, if it's open, priority, list of migrations for this PR

- Prioritize a pull request:
  `.skeefree prioritize-pr <pr url> <urgent|high|normal|low>`
  Pull requests have `normal` priority by default.
  examples:
  - `.skeefree prioritize-pr https://github.com/myorg/my_repo/pull/12345 high`
  - `.skeefree prioritize-pr https://github.com/myorg/my_repo/pull/12345 normal`
  - `.skeefree prioritize-pr https://github.com/myorg/my_repo/pull/12345 low`
  Please use `urgent` sparingly.

- Forget a pull request. Whether this will be of use is to be seen. If `skeefree` probed and found a PR, you may ask `skeefree` to _forget it_ under the condition that none of its migrations have begun.
  `.skeefree forget-pr <pr url>`
  example:
  `.skeefree forget-pr https://github.com/myorg/my_repo/pull/123456`

### Migrations

A PR contains one or more migrations. A migration is uniquely identified by the PR URL and the name of the table (whether created, altered or dropped).

- Manually set a migration to _autorun_. This is useful for repositories where `autorun` is disabled and you want `skeefree` to run a specific migration.
  `.skeefree approve-autorun <pr url> <table>`
  example:`.skeefree approve-autorun https://github.com/myorg/myrepo/pull/12345 my_table`

  If the repository had `autorun` disabled, then the strategy for migrations is `manual`. A `approve-autorun` will change the strategy to either `direct` (for `CREATE` and `DROP` migrations) or `gh-ost` (for `ALTER` migrations).

- Retry a failed migration:
  `.skeefree retry-migration <pr url> <table>`
  example: `.skeefree retry-migration https://github.com/myorg/my_repo/pull/12345 sample_data`

  You may only retry migrations that are in `failed` state. Scenario: `skeefree` is running a multi-day migration on `some_table`. You choose to `.migration panic some_table` because of some required maintenance. `skeefree` will mark that migration as `failed` after a few moments. You will then be able to `retry-migration`. Note: you may need to `.mysql dummy-drop` an existing `_gho` table.

- Mark a migration as `complete`:
  `.skeefree mark-complete <pr url> <table>`
  example: `.skeefree mark-complete https://github.com/myorg/my_repo/pull/12345 sample_data`

  You may have executed some migration manually. It's done, and you want to let `skeefree` know.
  Scenario: some migration keeps failing because of some internal problem. You fix it manually.
  Scenario: some migration is so big it never completes. You choose to alter on replicas and promote new master.
