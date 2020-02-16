# How skeefree works: skeema-diff Action

`skeema-diff` uses GitHub Actions and [skeema](https://github.com/skeema/skeema) to infer the migration statements in a migration PR.

The general outline is:

- `skeema-diff` Action runs on "Pull Request", meaning when the PR is created and any time a new commit comes in. And only when the PR is into `master`.
- Utilizes the built-in MySQL server available in GitHub Actions
- Checks out `master` branch, pushed the schema in `master` into the container's MySQL
- Checks out the PR branch, uses `skeema` to compare the branch schema with the container's MySQL.
- If no differences are found, nothing happens.
- Generates migration statements (`CREATE`, `DROP`, `ALTER`). Adds as PR comment, updates original PR body
- Creates a `migration:skeema:diff` label. This assists in discovery of this PR.

The Action re-evaluates upon any further incoming commit. It will regenerate the migration statements (or remove them altogether if no schema diff is found).

### The use of Actions

Actions are a recent addition (at the time of writing `skeefree`) in GitHub's functionality. The reason we're using an Action to generate the diff statements is:

- They have trivial access to the repo
- An Action will trivially fire on every PR commit
- An Action is a great way to get `skeema` to run in a PR's context without forcing repos to actually contain the `skeema` binary
- We can use the good-old PR flow, and utilize GitHub's reviews, comments, labels.

Actions have limitations:

- The run in a container in Azure. They have no access to our infrastructure.
  - At this time on-premise Actions are still in early-testers phase. We do not have the time to spare to wait on this.

As result, an action has no access to our production databases, to the `skeefree` k8s app, to Sites API (internal service), to `mysql-discovery` (internal service), etc. It can only recognize changes in the scope of the PR and of its repo, in general.

### The use of skeema

`skeema` is an actively developed open source tool. It's objective is to operate schema management cycles. It can both diff schema changes as well as apply them. However:

- `skeema` integrates with `pt-online-schema-change` and with `gh-ost`. However, we do not use `skeema` to invoke `gh-ost`.
  - Right now `skeema` runs from within the Action scope, and obviously cannot run anything in production, let alone multi-day migrations.
  - If we were to then extract `skeema` from Actions and run it from the outside, we'd be duplicating flow.
  - We'd still be unable to run `skeema` on our `k8s` servers: remember, schema changes can run for hours and days.
  - We want to be able to run concurrent migrations (on different clusters), and to schedule migrations; so, much of the logic we already need to take care of, ourselves.
  - We were building on top of already-existing-and-proven infrastructure. If we were to build `skeefree` from scratch, the design may have been different.
- Entirely possible `skeema` offers something more that we could use, but the above illustrates how we got here.

And so we only use `skeema` up to the point it generates _diff_ statements. This is no small feat. `skeema` is doing this job fantastically:

- It makes a lot of sense to `git` users
- It correctly identifies diffs
- It correctly generates migration statements
- Statement are generated in canonical, formal format. This makes parsing of migration statements very reliable.
