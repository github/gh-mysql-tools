# skeefree

Automated schema migrations for `github/*` repos

`skeefree` is an app which collaborates with other components to achieve automated schema migration flow at GitHub. The complete flow is composed of:

- [GitHub Actions](https://github.com/features/actions): an Action runs on Pull Request to identify if and which schema changes are pending
- [skeema](https://github.com/skeema/skeema): an open source tool, which we use to identify which schema change is pending, and generate a formal statement to transition into the new schema
- [gh-ost](https://github.com/github/gh-ost): our online schema migration tool, which runs reliable, auditable, controllable migrations on our busy clusters
- `skeefree`: this repo, a service (internally deployed on kubernetes) which interacts the schema changes Pull Requests (by collaborating with the Action), which supports chatops for control and visibility, and which can kick the schema change, either directly (`CREATE TABLE`, `DROP TABLE`) or indirectly (invoke `gh-ost` to run the migration).

For more information, read [How skeefree works](docs/how.md)


## Deployment

We deploy `skeefree` in two forms:

- A service (internally at GitHub we run this on kubernetes)
- A binary deployed to "utility" hosts where we run `gh-ost` on. The binary helps `gh-ost` interact in the `skeefree` flow:
  - It activates `gh-ost`
  - And gets called by `gh-ost` via _hooks_.


## Docs

- [Docs](docs/)
