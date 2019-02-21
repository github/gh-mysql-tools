# gh-mysql-rewind

Move MySQL back in time, decontaminate or un-split brain a MySQL server, restore it into replication chain.

### Objective

Rewinding MySQL is a way to move MySQL back in time, so as to de-apply bad transactions, and set the server as a valid replica to some other server.

There are two use cases:

- A replica accidentally contaminated by DML, e.g. some `DELETE` was executed directly on replica.
- A split brain master: a scenario where a failover process demoted master A and promoted master B, even as demoted master A continued to receive some traffic. `gh-mysql-rewind` can un-split brain on server A and restore it as a healthy replica under B or any of its healthy replicas.

### MySQL requirements

- MySQL GTID replication (`gtid_mode=ON`)
- `binlog_format=ROW`
- `binlog_row_image=FULL`
- Binary logs enabled
- `log_slave_updates` enabled
- Tested on MySQL 5.7, should work with 5.6, 8.0


### Public use / requirements

`gh-mysql-rewind` is developed internally at GitHub and released to the public under the MIT license, and in slightly modified form (removing GitHub-specific code).

As an internal tool, it uses some of our existing infrastructure. Users of this tool should take a few steps before executing on their environments:

- Install `orchestrator`, or alternatively replace all `orchestrator-client` commands with their own implementation.
- Place the MariaDB `mysqlbinlog` binary, version `10.3.10` or above, on the server to be rewinded
- Setup MySQL credentials for the tool to use
- All required and suggested changes are indicated in the code. Look for `IMPLEMENTATION`.

### General overview

`gh-mysql-rewind` is an implementation that utilizes two technologies:

- Oracle/MySQL GTID
- MariaDB's `mysqlbinlog --flashback`

With GTID, we are able to know what went wrong on a server by observing the _errant transactions_: GTID entries applied on a server but not on its master or would-be master. That is the general detection mechanism.

With `mysqlbinlog --flashback` we are able to generate the anti-chain-of-events in a binary log. Applying that onto a server effectively rewinds it back in time. Mostly.

The problem is that MariaDB is agnostic of MySQL-GTID. `mysqlbinlog --flashback` ignores any GTID info and generates no GTID info.

`gh-mysql-rewind` bridges the two technologies. It uses GTID to detect which binary logs contain the offending transactions, then uses `flashback` to de-apply those transactions, and finally does the math to fix `executed_gtid_set`, `gtid_purged`.

### Usage

```
gh-mysql-rewind -m <[intermediate-]master-host> [-x] [-r]
  Rewind errant transactions on local server and rewire to replicate from master-host
  -m master-host, serves as GTID baseline
  -x execute (default is noop)
  -r auto-start replication upon rewiring
```

- `gh-mysql-rewind` needs to run on the corrupted server.
- Needs to be executed by a user with `sudo` privileges.
- Needs `orchestrator-client` to be available.
- `master-host` must be provided. This will be a "good" server in the same cluster as the corrupted server. Not necessarily a `master`. `gh-mysql-rewind` will use `master-host` to infer the errant transactions, and the operated box will end up replicating from `master-host`.
- Sanity/protection checks:
  - Server must be `read-only`, to avoid running on an active master.
  - Must have no replicas (`gh-mysql-rewind` will issue a `RESET MASTER`).
  - Must not be actively replicating.
  - Must not use `SQL_DELAY`.
  - Must have some errant GTID

### Deep dive

The tool needs to:

- Identify which binary logs need to be reverted
- Actually revert those binlogs
- Keep accurate track of reverted GTID entries, reconfigure `gtid_purged` on server.

Flow breakdown:

- Sanity checks.
- Note down `executed_gtid_set` on server.
- Note down `executed_gtid_set` on master.
- Compute errant GTID on server.
- Sanity check: there actually is errant GTID.
- Identify which binary logs contain the errant GTID.
  - Will revert the last `n` (`n >= 1`) binary logs of the server. e.g. if binary logs are `mysql-bin.001, mysql-bin.002, mysql-bin.003, mysql-bin.004, mysql-bin.005`:
    - if `mysql-bin.005` contains all errant transactions, then only `mysql-bin.005` is reverted.
    - if `mysql-bin.003` and `mysql-bin.004` contain all errant transactions, then `mysql-bin.005` is reverted, then `mysql-bin.004`, then `mysql-bin.003`.
  - Calculate the entire GTID set contained by those binary logs, by manually parsing the binary logs
- Generate `flashback` for the relevant binary logs.
  - Inject dummy GTID statements into `flashback` output (which is originally ignorant of GTID)
- Apply flashback onto MySQL
- `RESET MASTER`
- `set global gtid_purged=?`, by subtracting: original `executed_gtid_set` - reverted GTID set.
- Clearing relay logs (existing relay logs are inconsistent with the position the server needs to replicate from).
- Reconfigure replication.
- Potentially resume replication (if `-r` is provided).

### Limitations

- *DDL DANGER:* `gh-mysql-rewind` cannot undo DDLs. If a `ALTER TABLE` takes place, `gh-mysql-rewind` will rewind MySQL back to the past across said DDL, but will not actually de-apply the DDL. As result, once the server resumes replication it is likely to break on the DDL (e.g. it won't be able to drop an index because the index is already dropped).
Some DDLs will possibly just NotWork™. Like a `DROP COLUMN` or `ADD COLUMN` closely coupled with operations on the table. There would be a mismatch in the number of columns when reverting events.

- Does not support `JSON`, `POINT` data types and will break when trying to flashback a statement which includes tables with such columns.

- `gh-mysql-rewind` operates on entire binlog files. This can be improved upon, but it simplifies the process. A complete binary log is the smallest amount of rewind. This means we probably rewind more than strictly necessary. The downside is that we spend time reverting events we don't need to revert, and then spend time reapplying those events.

- The operated server must have no replicas: the operation ends up with a `RESET MASTER`. If multiple servers need to be rewinded, begin with leaf nodes and work your way up, one by one. Alternatively, rearrange the topology such that your operated server has no replicas (e.g. use `.orc relocate-replicas <operated-server> below <some-other-server>`)

### Visual walkthrough

Let's assume the worst scenario, a split brain. Before trouble began, the topology looked like this:

```
m-old
+ r1
+ r2
  + r3
+ m-new
  + r4
    + r5
  + r6
```

A network partition caused a failover and a splitting of the topology into:

```
m-old
+ r1
+ r2
  + r3

m-new
+ r4
  + r5
+ r6
```

Production traffic has been directed to `m-new`, the newly promoted master, and to `r4, r5, r6`, its replicas.

Unfortunately `m-old` was receiving writes from local apps even after the failover. This leaves `m-old, r1, r2, r3` in a split brain state.

- We want to run `gh-mysql-rewind` on all four boxes.
- We cannot immediately start with `m-old` nor with `r2` because they have replicas.
  If we moved away their replicas then we'd be able to operate on them.
- We can start with `r1` and `r3`.
- We can point them to _any_ of `m-new, r4, r5, r6` assuming, of course, there's `log-bin=1` and `log-slave-updates` on those servers.
- For example, we'd login to `r3` and run: `gh-mysql-rewind -m r5 -x -r`. If all goes well, this will lead to:

```
m-old
+ r1
+ r2

m-new
+ r4
  + r5
    + r3
+ r6
```

- For example, we can then login to `r2` (which now does not have replicas) and run: `gh-mysql-rewind -m m-new -x -r`. If all goes well, this will lead to:

```
m-old
+ r1

m-new
+ r2
+ r4
  + r5
    + r3
+ r6
```

- And so forth until we've rewinded all corrupted servers.

#### Rewinding concurrently

Back to the split brain state in the above:

```
m-old
+ r1
+ r2
  + r3
```
- It's OK to rewind `r1` and `r3` concurrently. It's OK to point both to same `master-host` and it's OK to point them to different master hosts.
- `r2` cannot be rewinded as long as `r3` is replicating from it.
- You may `.orc relocate r3 below m-old`, to get:
  ```
  m-old
  + r1
  + r2
  + r3
  ```
  and then it's OK to rewind all three `r1, r2, r3` concurrently.

### Testing

`gh-mysql-rewind` is tested internally at GitHub.

### External links

- `gh-mysql-rewind` FOSDEM presentation [video](https://www.youtube.com/watch?v=UL--ew3n3QI) and [slides](https://speakerdeck.com/shlominoach/un-split-brain-mysql)
- [MariaDB flashback](https://mariadb.com/kb/en/library/flashback/)
- [orchestrator](https://github.com/github/orchestrator) project, binary [releases](https://github.com/github/orchestrator/releases).
