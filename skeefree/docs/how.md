# How skeefree works

End-to-end, the migration flow is complex. It begins with a developer making local changes, continues with a PR, approving teams, onto automated schema change detection, through running the migration in production, logging and notifications on the PR and in chat, to full completion.

Some of the operations depicted in the above are best dealt with using designated processes. For example:

- Since the developer opens a PR, it's best to use the GitHub flow (request review, review, add commits, approval)
- [Chatops](chatops.md) are formalized and managed well within Moda apps
- Much of the information is obtained from our existing infrastructure, such as Sites API (internal to GitHub), MySQL-Discovery (internal to GitHub), and more.
- Online migrations (any `ALTER TABLE`) are only executed by [gh-ost](https://github.com/github/gh-ost)

As result, the complete flow is not owned by any single process. The flow is a coordination of efforts by multiple processes, communicating with each other in different ways. There are three main actors in the flow:

- [skeema-diff](how-action.md): a GitHub Action utilizing [skeema](https://github.com/skeema/skeema) to generate diff statements and annotate with labels
- [skeefree](how-skeefree.md): this repo, acting as the controller; detects PRs, puts context to the migration, runs the migrations, provides visibility and control via chatops.
- [gh-ost](how-gh-ost.md): our schema migration tool, uses a `skeefree` binary together with hooks, to read schema migration assignments, execute and update status.

# How the flow looks to the GitHub engineers

The flow is design to reduce human interaction to minimum, while maintaining the conventional Pull Request flow.

Pre-requisites:

- Developers will use `skeema` schema notation in their repositories; schema changes are made by using the GitHub Pull Request flow.
- A repo is submitted _once_ to `skeefree` via `.skeefree add-repo` chatop.
- The `skeema-diff` Action file is added to the repo.

### The flow, TL;DR

- A developer creates a PR. Gets owner team to approve, is happy with the changes.
- `skeema-diff` Action analyses the changes.
- The developer adds `migration:for:review` label.
- `skeefree` kicks in, generates migration commands
- `skeefree` requests review from databases team
- databases team member approves PR
- `skeefree` automatically runs the migration, comments as needed
- When all migrations complete, `skeefree` @-mentions the developer with instructions to deploy/merge.

### The flow, full description

- A developer submits a PR on their repo. The PR happens to have a schema change (otherwise this flow is irrelevant).
- The `skeema-diff` Action springs to life, diffing the PR's schema with the `master`'s
  - Generating schema change comment (`ALTER`, `CREATE`, `DROP`)
  - Adding `migration:skeema:diff` label.
  - Adding a comment telling the developer that, when they're ready, they should add the `migration:for:review` label
- The developer and their team review the PR. They may look at the Action output to validate the change.
- The PR is approved by the development team.
- The developer wishes to proceed. They add the `migration:for:review` label.
- `skeefree` routinely probes approved PRs with `migration:skeema:diff` and `migration:for:review` labels. `skeefree`:
  - Detects the PR
  - Generates migration commands, adds as comment
  - Adds `migration:skeefree:detected` label
  - Requests review from databases team.
- databases team member reviews using the normal PR review cycle; changes can be added, in which case both `skeema-diff` Action and `skeefree` re-generate migration statements and add new PR comments.
- databases team member approves the PR
- `skeema` auto-detects that the PR is approved by databases team, and proceeds to deploy:
  - Adds label `migration:skeefree:queued`
  - Runs the migration(s) however it sees fit.
    This step is complex, and may take time. But as far as the GitHub engineer cares, it's a black box.
  - Ideally all migrations (a PR may have more than one migration) are successful, `skeefree` reports back via comment on completion or failure of each migration.
- When all PR migrations are `complete`, `skeefree` comments as much, and instructs the developer to follow the deploy/merge process.
- The developer deploys and merges.
- :pizza:

To focus on the involvement of the various teams:

Application developers:
- Submit PR
- Review, approve
- Add `migration:for:review` label
- Wait for databases team approval
- Magic happens; they get notified when the migration is complete
- deploy & merge.

databases team:
- Requested to review when a migration PR is approved; are told by `skeefree` exactly how the migration would run and what it would do
- Approve
- Ideally all migrations are successful (on failure: different discussion)
