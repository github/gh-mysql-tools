# How skeefree works: gh-ost

`gh-ost` migrations are executed on utility hosts.

A cronjob runs every minute on the utility servers, that kicks a check: do we need to run a migration?

What runs the logic is `skeefree` CLI. It's the same `skeefree` build, but does not run as a service.

The flow is:

- Once per minute, run `gh-migrate-skeefree` (internal script)
- which calls `skeefree -c own-migration`. This attempts to find a migration in `ready` [state](migrations.md) and own it. Multiple utility hosts will compete for such migration; one and only one will win.
- The output of `skeefree -c own-migration` is a JSON which, among other things, contains the actual `command-to-run-gh-ost ...`. The script executes that command.
- Now `gh-ost` is running, and is normally calling hooks.
- In four of the hooks: `startup`, `status`, `success`, `failure`, we call upon `skeefree` CLI to take action based on the hook type. For example, on `startup` we will add a PR comment noting that migration has started. On `complete` we will set the migration to `complete` state, and add a PR comment.
