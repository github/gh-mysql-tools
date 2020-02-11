# Adding a new repository

Repositories need to opt-in to `skeefree` to enjoy automated migrations.

# Basic requirements

0. Repo is in the org supported by `skeefree` (see `config.go`)
0. You have an API token with `Admin` access on your org
0. databases team has `Write` access to the repo (otherwise `skeefree` cannot request them to review)
0. Must be using `skeema` to manage schema definitions (see next)

# Steps

0. Confirm basic requirements above
0. Create a PR to configure `[skeema-diff-ci]` for all `.skeema` schemas. See [.skeema path](#skeema-path)
0. ^ is merged
0. Create a PR to add the `.github/workflows/skeema-diff.yml` Action file. See [.skeema path](#skeema-diff-action)
0. ^ is merged
0. `.skeefree add-repo myorg/<the-repo> <owner-team>`
0. for each of the schemas managed by this repo (one or more):
  - `.skeefree repo-map myorg/<the-repo> <skeema_schema_name> <cluster_name> <production_schema_name>`
0. enable auto-migrations: `.skeefree repo-autorun enable myorg/<the-repo>`
  - potentially `.skeefree approve-autorun <PR URL> <table_name>` for a few migrations already identified on this repo which which have been assigned the `manual` strategy as per `.skeefree sup`.

# repo requirements

The repo must be using [skeema](https://github.com/skeema/skeema) to manage schema definition.

How the team use `skeema` internally on their dev/staging hosts is up to them and is irrelevant to `skeefree`. We basically care that the schema is versioned in `git` and that there's a `.skeema` definition file.

There are multiple ways in how to place `.skeema` file and how to split configuration between `.skeema` files.
- There can be a single `.skeema` file (call it "top-level"), or
- multiple `.skeema` files per directory/schema, with one top-level `.skeema` file

## .skeema path

Top-level `.skeema` file is expected to be located in one of the following paths under the repo's root path:
  - `./` (root directory)
  - `skeema/`
  - `schemas/`
  - `schema/`
  - `db/`
  `skeefree` will auto-detect `.skeema` file if in either of those locations.

If `.skeema` is in some other, arbitrary path, use the file [skeema-diff.cfg](../.github/skeema-diff.cfg) to indicate said path.

## .skeema config
Add the following section to repo's top-level `.skeema` file:
```
[skeema-diff-ci]
host=127.0.0.1
port=3306
user=root
```
  This is configured to work with the MySQL server available on a GitHub Actions environment.

Depending on whether the repo uses a single `.skeema` file or multiple `.skeema` files, add the following in the appropriate file:
```
[skeema-diff-ci]
schema=skeema:<production_schema_name>
```

Do so for any schema managed by this repo.

`skeema-diff` CI communicates information to `skeefree` by way of PR comments and hints. It may look like this:

```
-- skeema:diff
-- skeema:ddl:use `skeema:my_repo`;
-- skeema:ddl:begin
ALTER TABLE `my_table` ADD KEY `my_index1` (`some_column1`), ADD KEY `my_index2` (`some_column2`);
-- skeema:ddl:end
```

The `use` comment is how `skeema-diff` communicates the schema to `skeefree`. In the above example, the entry `schema=skeema:my_repo` makes `skeema-diff` communicate `skeema:my_repo` to `skeefree`.

`skeefree` itself strips off the `skeema:` prefix (in fact, it strips any prefix with `:`). And so `skeefree` sees `my_repo`. This is in fact the name of the schema in `production`.

We _choose this as convention_. The entry `skeema:<production_schema_name>` gives us:

- Information about the actual schema name used in `production`, while
- not actually using the production schema name (this is bad practice). Prefixing with `skeema:` makes it clear that this isn't and cannot be a production schema.

# skeema-diff action

After `.skeema` file(s) changes (see above) are merged, create a new file, `.github/workflows/skeema-diff.yml`, to serve as a GitHub Action.
Copy the contents of `skeema-diff.yml` into the new file. `skeefree` is the source of truth for `skeema-diff.yml`, and all repos should be using this file. Updates to this file will be distributed manually, as needed, to all repos.

Create a PR with this file. Your PR will now include a `skeema-diff / skeema-diff (pull_request)` test.

CI should pass without indicating any schema changes. If it does generate schema diff statements, seen as new PR comment and a `migration:skeema:diff` label, then something is misconfigured. Likely `schema` is not in the right place.

# Verifying behavior

For testing purposes, create a PR with a dummy schema change in the repo. It's enough that you add/remove a column to/from some table. Commit and push, create a **Draft PR**.

If all works well, the `skeema-diff` Action will identify the change during CI build, and will:

- Add a PR comment indicating the schema change
- Add a `migration:skeema:diff` label
