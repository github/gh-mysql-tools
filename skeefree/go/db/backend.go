package db

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/github/skeefree/go/config"
	"github.com/github/skeefree/go/core"
	"github.com/github/skeefree/go/util"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// Backend takes care of all backend database requests. All database queries will go here.
// I chose to implement this with a `Backend` struct that has methods, be we could have just the same
// used global functions (scoped by this package of course).

// We use sqlx, which is a popular library on top of golang's standard database/sql package,
// and which for example can map query columns onto struct fields.

const maxConnections = 3
const electionExpireSeconds = 5

const electionInterval = time.Second
const stateInterval = 10 * time.Second

type Backend struct {
	db *sqlx.DB

	serviceId   string
	leaderState int64
	healthState int64
}

func NewBackend(c *config.Config) (*Backend, error) {
	db, err := sqlx.Open("mysql", mysqlRWConfigDSN(c))
	if err != nil {
		return nil, err
	}
	serviceId, err := util.HostnameBasedToken()
	if err != nil {
		return nil, err
	}
	return &Backend{
		db:        db,
		serviceId: serviceId,
	}, nil
}

func mysqlRWConfigDSN(c *config.Config) string {
	cfg := mysql.NewConfig()
	cfg.User = c.MysqlRwUser
	cfg.Passwd = c.MysqlRwPass
	cfg.Net = "tcp"
	cfg.Addr = c.MysqlRwHost
	cfg.DBName = c.MysqlSchema
	cfg.ParseTime = true
	cfg.InterpolateParams = true
	cfg.Timeout = 1 * time.Second

	return cfg.FormatDSN()
}

func (backend *Backend) ServiceId() string {
	return backend.serviceId
}

// ContinuousElections is a utility function to routinely observe leadership state.
// It doesn't actually do much; merely takes notes.
func (backend *Backend) ContinuousElections(logError func(error)) {
	electionsTick := time.Tick(electionInterval)

	for range electionsTick {
		if err := backend.AttemptLeadership(); err != nil {
			logError(err)
		}
		newLeaderState, _, err := backend.ReadLeadership()
		if err == nil {
			atomic.StoreInt64(&backend.healthState, 1)
			if newLeaderState != backend.leaderState {
				backend.onLeaderStateChange(newLeaderState)
				atomic.StoreInt64(&backend.leaderState, newLeaderState)
			}
		} else {
			atomic.StoreInt64(&backend.healthState, 0)
			// and maintain leader state: graceful response to backend errors
			logError(err)
		}
	}
}

func (backend *Backend) AddRepository(r *core.Repository) (added bool, err error) {
	result, err := backend.db.Exec(`
    insert ignore into repositories
      (org, repo, owner, autorun, added_timestamp, updated_timestamp)
    values
      (?, ?, ?, ?, now(), now())
    `, r.Org, r.Repo, r.Owner, r.AutoRun)
	if err != nil {
		return added, err
	}
	if affected, _ := result.RowsAffected(); affected > 0 {
		r.Id, err = result.LastInsertId()
		return true, err
	}
	return false, err
}

func (backend *Backend) UpdateRepository(r *core.Repository) (updated bool, err error) {
	result, err := backend.db.Exec(`
    update repositories set
      owner=?,
			autorun=?,
			updated_timestamp=now()
		where
			org=?
			and repo=?
    `, r.Owner, r.AutoRun, r.Org, r.Repo)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (backend *Backend) DeleteRepository(r *core.Repository) (deleted bool, err error) {
	result, err := backend.db.NamedExec(`
    delete from repositories where
      id=:id
			and org=:org
			and repo=:repo
    `, r)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (backend *Backend) ReadRepository(r *core.Repository) (err error) {
	queryBase := `select id, org, repo, owner, autorun, added_timestamp, updated_timestamp from repositories where`
	if r.Id > 0 {
		return backend.db.Get(r, fmt.Sprintf("%s id=?", queryBase), r.Id)
	}
	if r.HasOrgRepo() {
		return backend.db.Get(r, fmt.Sprintf("%s org=? and repo=?", queryBase), r.Org, r.Repo)
	}
	return fmt.Errorf("ReadRepository: no Id, no Org/Repo; cannot read repository")
}

func (backend *Backend) ReadRepositories() (repos []core.Repository, err error) {
	query := `select id, org, repo, owner, autorun, added_timestamp, updated_timestamp from repositories order by org, repo`
	err = backend.db.Select(&repos, query)
	return repos, err
}

func (backend *Backend) WriteRepositoryMapping(m *core.RepositoryProductionMapping) (err error) {
	_, err = backend.db.Exec(`
    replace into repo_production_mapping
      (org, repo, hint, mysql_cluster, mysql_schema, added_timestamp, updated_timestamp)
    values
      (?, ?, ?, ?, ?, now(), now())
    `, m.Org, m.Repo, m.Hint, m.MySQLCluster, m.MySQLSchema)

	return err
}

func (backend *Backend) RemoveRepositoryMapping(m *core.RepositoryProductionMapping) (err error) {
	_, err = backend.db.NamedExec(`
    delete from repo_production_mapping where
			org=:org
			and repo=:repo
			and hint=:hint
    `, m)
	return err
}

func (backend *Backend) ReadRepositoryMappings(r *core.Repository) (mappings []core.RepositoryProductionMapping, err error) {
	query := `
		select
			id, org, repo, hint, mysql_cluster, mysql_schema, added_timestamp, updated_timestamp
		from
			repo_production_mapping
		where
			org=?
			and repo=?
		order by
			hint
		`
	err = backend.db.Select(&mappings, query, r.Org, r.Repo)
	return mappings, err
}

func (backend *Backend) SubmitPR(pr *core.PullRequest) (err error) {
	sqlResult, err := backend.db.NamedExec(`
	    update pull_requests set
				title=:title,
				author=:author,
				priority=:priority,
				status=:status,
				is_open=:is_open,
				requested_review_by_db_reviewers=:requested_review_by_db_reviewers,
				approved_by_db_reviewers=:approved_by_db_reviewers,
				requested_review_by_db_infra=:requested_review_by_db_infra,
				approved_by_db_infra=:approved_by_db_infra,
				label_diff:=label_diff,
				label_detected=:label_detected,
				label_queued=:label_queued,
				label_for_review=:label_for_review,
				probed_timestamp=now()
			where
				org=:org
				and repo=:repo
				and pull_request_number=:pull_request_number
    `, pr)
	if err != nil {
		return err
	}
	affected, err := sqlResult.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		// good, row updated
		return nil
	}
	// No rows affected? try and isnert the row
	_, err = backend.db.NamedExec(`
	    insert into pull_requests (
				org, repo, pull_request_number, title, author, priority, status, is_open,
				requested_review_by_db_reviewers,
				approved_by_db_reviewers,
				requested_review_by_db_infra,
				approved_by_db_infra,
				label_diff,
				label_detected,
				label_queued,
				label_for_review,
				probed_timestamp
			) values (
				:org, :repo, :pull_request_number, :title, :author, :priority, :status, :is_open,
				:requested_review_by_db_reviewers,
				:approved_by_db_reviewers,
				:requested_review_by_db_infra,
				:approved_by_db_infra,
				:label_diff,
				:label_detected,
				:label_queued,
				:label_for_review,
				now()
			)
    `, pr)
	return err
}

func (backend *Backend) SubmitPRStatements(pr *core.PullRequest, statements []string) (err error) {
	tx, err := backend.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, statement := range statements {
		if _, err := tx.Exec(`
		    insert into pull_request_migration_statements (
					pull_requests_id, migration_statement, status
				) values (
					?, ?, ?
				)
	    `, pr.Id, statement, string(core.PullRequestMigrationStatementStatusSuggested)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (backend *Backend) ForgetPR(pr *core.PullRequest) (err error) {
	if pr.Id == 0 {
		return fmt.Errorf("Cannot forget PR %s as it has no internal Id", pr.String())
	}

	tx, err := backend.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Sanity check: cannot forget PR if it has migrations that are imminent, running, or complete
	var statuses []string
	if err := backend.db.Select(&statuses, `select status from migrations where pull_requests_id=? for update`, pr.Id); err != nil {
		return err
	}
	for _, status := range statuses {
		switch core.MigrationStatus(status) {
		case
			core.MigrationStatusReady,
			core.MigrationStatusRunning,
			core.MigrationStatusComplete,
			core.MigrationStatusUnknown:
			{
				return fmt.Errorf("cannot forget PR: migration found with '%s' status.", status)
				// transaction is rolled back
			}
		}
	}

	if _, err := tx.Exec(`delete from pull_requests where id=?`, pr.Id); err != nil {
		return err
	}
	if _, err := tx.Exec(`delete from pull_request_migration_statements where pull_requests_id=?`, pr.Id); err != nil {
		return err
	}
	if _, err := tx.Exec(`delete from migrations where pull_requests_id=?`, pr.Id); err != nil {
		return err
	}

	return tx.Commit()
}

func (backend *Backend) ForgetPRStatementsAndMigrations(pr *core.PullRequest) (err error) {
	if pr.Id == 0 {
		return fmt.Errorf("Cannot forget statements and migrations for PR %s as it has no internal Id", pr.String())
	}

	tx, err := backend.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`update pull_request_migration_statements set status=? where pull_requests_id=?`, string(core.PullRequestMigrationStatementStatusCancelled), pr.Id); err != nil {
		return err
	}
	if _, err := tx.Exec(`update migrations set status=? where pull_requests_id=? and status in (?, ?)`, string(core.MigrationStatusCancelled), pr.Id, string(core.MigrationStatusProposed), string(core.MigrationStatusQueued)); err != nil {
		return err
	}

	return tx.Commit()
}

func (backend *Backend) UpdatePRPriority(pr *core.PullRequest, priority core.PullRequestPriority) (countAffected int64, err error) {
	if pr == nil {
		return countAffected, fmt.Errorf("UpdatePRStatus: nil pr")
	}
	if pr.Id == 0 {
		return countAffected, fmt.Errorf("UpdatePRStatus: pr.Id == 0")
	}
	sqlResult, err := backend.db.Exec(`
		update pull_requests set priority=? where id=?
		`, int(priority), pr.Id,
	)
	if err != nil {
		return countAffected, err
	}
	return sqlResult.RowsAffected()
}

func (backend *Backend) ReadPR(pr *core.PullRequest) (err error) {
	err = backend.db.Get(pr, `
    select
				*
			from pull_requests
			where
				org=? and repo=? and pull_request_number=?
    `, pr.Org, pr.Repo, pr.Number)
	if err != nil {
		return fmt.Errorf("PR %s not found in database", pr.URL())
	}
	return nil
}

func (backend *Backend) ReadActivePRs() (prs []core.PullRequest, err error) {
	err = backend.db.Select(&prs, `
    select * from pull_requests where status in (?, ?)
    `, core.PullRequestStatusDetected, core.PullRequestStatusQueued)
	return prs, err
}

func (backend *Backend) ReadOpenPRs() (prs []core.PullRequest, err error) {
	err = backend.db.Select(&prs, `select * from pull_requests where is_open=1`)
	return prs, err
}

func (backend *Backend) ReadNonCompletedPRsWithCompletedMigrations() (prs []core.PullRequest, err error) {
	err = backend.db.Select(&prs, `
    select
			pull_requests.*
		from
			pull_requests
			join migrations on (pull_requests.id = migrations.pull_requests_id)
		where
			pull_requests.is_open=1
			and pull_requests.status!=?
		group by
			pull_requests.id
		having
			count(*) > 0
			and sum(migrations.status in (?,?)) = count(*)
    `,
		core.PullRequestStatusComplete,
		core.MigrationStatusComplete,
		core.MigrationStatusCancelled,
	)
	return prs, err
}

func (backend *Backend) ReadPullRequestMigrationStatements(pr *core.PullRequest) (statements []core.PullRequestMigrationStatement, err error) {
	err = backend.db.Select(&statements, `
    select
			id, pull_requests_id, migration_statement, status, added_timestamp
		from pull_request_migration_statements
		where
			pull_requests_id=?
			and status!=?
    `, pr.Id, string(core.PullRequestMigrationStatementStatusCancelled))
	return statements, err
}

func (backend *Backend) SubmitMigrations(migrations []*core.Migration) (countSubmitted int64, err error) {
	for _, m := range migrations {
		sqlResult, err := backend.db.Exec(`
		    insert ignore into migrations (
					org,
					repo,
					pull_request_number,
					pull_requests_id,
					pull_request_migration_statements_id,
					mysql_cluster,
					mysql_shard,
					mysql_schema,
					mysql_table,
					migration_statement,
					alter_statement,
					suggestion,
					canonical,
					strategy,
					token,
					status,
					added_timestamp
				) values (
					?, ?, ?, ?,
					?, ?, ?, ?,
					?, ?, ?, ?,
					?, ?, ?, ?,
					now()
				)
	    `,
			m.PR.Org,
			m.PR.Repo,
			m.PR.Number,
			m.PR.Id,
			m.PRStatement.Id,
			m.Cluster.Name,
			m.Shard,
			m.Repo.MySQLSchema,
			m.TableName,
			m.PRStatement.Statement,
			m.Alter,
			m.Suggestion,
			m.Canonical,
			string(m.Strategy),
			m.Token,
			string(m.Status),
		)
		if err != nil {
			return countSubmitted, err
		}
		rowsAffected, err := sqlResult.RowsAffected()
		if err != nil {
			return countSubmitted, err
		}
		countSubmitted += rowsAffected
	}
	return countSubmitted, err
}

// ReadNonCancelledMigrations reads all migration with status!='cancelled' for all open PRs (if given pr is nil)
// or for a specific PR if given.
func (backend *Backend) readMigrations(condition string, args ...interface{}) (migrations []core.Migration, err error) {
	query := fmt.Sprintf(`
	    select
				pull_requests.id,
				pull_requests.org,
				pull_requests.repo,
				pull_requests.pull_request_number,
				pull_requests.title,
				pull_requests.author,
				pull_requests.priority,
				pull_requests.status,
				pull_requests.is_open,
				pull_requests.requested_review_by_db_reviewers,
				pull_requests.approved_by_db_reviewers,
				pull_requests.requested_review_by_db_infra,
				pull_requests.approved_by_db_infra,
				pull_requests.label_diff,
				pull_requests.label_detected,
				pull_requests.label_queued,
				pull_requests.label_for_review,

				migrations.id,
				migrations.pull_request_migration_statements_id,
				migrations.mysql_cluster,
				migrations.mysql_shard,
				migrations.mysql_schema,
				migrations.mysql_table,
				migrations.migration_statement,
				migrations.alter_statement,
				migrations.suggestion,
				migrations.canonical,
				migrations.strategy,
				migrations.token,
				migrations.status
			from
				pull_requests
				join migrations on (pull_requests.id = migrations.pull_requests_id)
			where
				migrations.status!=?
				%s
			order by
				pull_requests.priority desc,
				pull_requests.id asc
			`, condition)
	args = append([]interface{}{core.MigrationStatusCancelled}, args...)

	rows, err := backend.db.Queryx(query, args...)
	if err != nil {
		return migrations, err
	}
	for rows.Next() {
		m := core.NewEmptyMigration()
		err = rows.Scan(
			&m.PR.Id,
			&m.PR.Org,
			&m.PR.Repo,
			&m.PR.Number,
			&m.PR.Title,
			&m.PR.Author,
			&m.PR.Priority,
			&m.PR.Status,
			&m.PR.IsOpen,
			&m.PR.RequestedReviewByDBReviewers,
			&m.PR.ApprovedByDBReviewers,
			&m.PR.RequestedReviewByDBInfra,
			&m.PR.ApprovedByDBInfra,
			&m.PR.LabeledAsDiff,
			&m.PR.LabeledAsDetected,
			&m.PR.LabeledAsQueued,
			&m.PR.LabeledForReview,

			&m.Id,
			&m.PRStatement.Id,
			&m.Cluster.Name,
			&m.Shard,
			&m.Repo.MySQLSchema,
			&m.TableName,
			&m.PRStatement.Statement,
			&m.Alter,
			&m.Suggestion,
			&m.Canonical,
			&m.Strategy,
			&m.Token,
			&m.Status,
		)
		m.Repo.Org = m.PR.Org
		m.Repo.Repo = m.PR.Repo
		migrations = append(migrations, *m)
	}
	return migrations, err
}

func (backend *Backend) ReadNonCancelledMigrations(pr *core.PullRequest) (migrations []core.Migration, err error) {
	if pr == nil {
		// Read all open PRs
		return backend.readMigrations("and pull_requests.is_open=1")
	}
	// Read given PR
	if pr.Id == 0 {
		return migrations, fmt.Errorf("given PullRequest has no Id")
	}
	return backend.readMigrations("and pull_requests_id=?", pr.Id)
}

func (backend *Backend) ReadGhostReadyMigrations(tokenHint string) (migrations []core.Migration, err error) {
	return backend.readMigrations(
		"and token='' and migrations.strategy=? and migrations.status=? and (token_hint=? or timestampdiff(second, ready_timestamp, now()) >= 300)",
		string(core.MigrationStrategyGhost), string(core.MigrationStatusReady), tokenHint,
	)
}

func (backend *Backend) QueuePRMigrations(pr *core.PullRequest) (countAffected int64, err error) {
	tx, err := backend.db.Beginx()
	if err != nil {
		return countAffected, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`update pull_requests set status=? where status=? and id=?`, string(core.PullRequestStatusQueued), string(core.PullRequestStatusDetected), pr.Id); err != nil {
		return countAffected, err
	}
	if _, err := tx.Exec(`update pull_request_migration_statements set status=? where pull_requests_id=? and status=?`, string(core.PullRequestMigrationStatementStatusApproved), pr.Id, string(core.PullRequestMigrationStatementStatusSuggested)); err != nil {
		return countAffected, err
	}
	sqlResult, err := tx.Exec(`
		update migrations set status=? where pull_requests_id=? and status=?
		`, string(core.MigrationStatusQueued), pr.Id, string(core.MigrationStatusProposed),
	)
	if err != nil {
		return countAffected, err
	}
	countAffected, err = sqlResult.RowsAffected()
	if err != nil {
		return countAffected, err
	}
	return countAffected, tx.Commit()
}

func (backend *Backend) UpdatePRStatus(pr *core.PullRequest, toStatus core.PullRequestStatus) (countAffected int64, err error) {
	if pr == nil {
		return countAffected, fmt.Errorf("UpdatePRStatus: nil pr")
	}
	if pr.Id == 0 {
		return countAffected, fmt.Errorf("UpdatePRStatus: pr.Id == 0")
	}
	sqlResult, err := backend.db.Exec(`
		update pull_requests set status=? where id=?
		`, toStatus, pr.Id,
	)
	if err != nil {
		return countAffected, err
	}
	return sqlResult.RowsAffected()
}

func (backend *Backend) UpdatePRMigrationsStatus(pr *core.PullRequest, fromStatus, toStatus core.MigrationStatus, withStrategy core.MigrationStrategy) (countAffected int64, err error) {
	if pr == nil {
		return countAffected, fmt.Errorf("QueuePRMigrations: nil pr")
	}
	if pr.Id == 0 {
		return countAffected, fmt.Errorf("QueuePRMigrations: pr.Id == 0")
	}
	sqlResult, err := backend.db.Exec(`
		update migrations set status=? where pull_requests_id=? and status=? and strategy=?
		`, toStatus, pr.Id, fromStatus, withStrategy,
	)
	if err != nil {
		return countAffected, err
	}
	return sqlResult.RowsAffected()
}

func (backend *Backend) UpdateMigrationTokenHint(migration *core.Migration, tokenHint string) (err error) {
	if migration == nil {
		return fmt.Errorf("UpdateMigrationTokenHint: nil migration")
	}
	if migration.Id == 0 {
		return fmt.Errorf("UpdateMigrationTokenHint: migration.Id == 0")
	}
	_, err = backend.db.Exec(`update migrations set token_hint=? where id=?`, tokenHint, migration.Id)
	return err
}

func (backend *Backend) UpdateMigrationStatus(migration *core.Migration, fromStatus, toStatus core.MigrationStatus, withStrategy core.MigrationStrategy) (countAffected int64, err error) {
	if migration == nil {
		return countAffected, fmt.Errorf("UpdateMigrationStatus: nil migration")
	}
	if migration.Id == 0 {
		return countAffected, fmt.Errorf("UpdateMigrationStatus: migration.Id == 0")
	}
	updateClause := ""
	switch toStatus {
	case core.MigrationStatusReady:
		updateClause = ", ready_timestamp=now()"
	case core.MigrationStatusRunning:
		updateClause = ", liveness_timestamp=now(), started_timestamp=IFNULL(started_timestamp, now())"
	case core.MigrationStatusComplete:
		updateClause = ", liveness_timestamp=now(), completed_timestamp=now(), token=''"
	case core.MigrationStatusFailed:
		updateClause = ", token=''"
	}
	query := fmt.Sprintf(`
		update migrations set status=? %s where id=? and status=? and strategy=?
		`, updateClause)
	sqlResult, err := backend.db.Exec(query, toStatus, migration.Id, fromStatus, withStrategy)
	if err != nil {
		return countAffected, err
	}
	return sqlResult.RowsAffected()
}

func (backend *Backend) UpdateMigrationStrategy(migration *core.Migration, fromStrategy, toStrategy core.MigrationStrategy) (countAffected int64, err error) {
	if migration == nil {
		return countAffected, fmt.Errorf("UpdateMigrationStrategy: nil migration")
	}
	if migration.Id == 0 {
		return countAffected, fmt.Errorf("UpdateMigrationStrategy: migration.Id == 0")
	}
	sqlResult, err := backend.db.Exec(`
		update migrations set strategy=? where id=? and strategy=?
		`, toStrategy, migration.Id, fromStrategy,
	)
	if err != nil {
		return countAffected, err
	}
	return sqlResult.RowsAffected()
}

func (backend *Backend) ReadMigrationByToken(token string) (migration *core.Migration, err error) {
	if token == "" {
		return migration, fmt.Errorf("Empty token in ReadMigrationByToken")
	}
	migrations, err := backend.readMigrations("and migrations.token=?", token)
	if err != nil {
		return migration, err
	}
	if len(migrations) == 0 {
		return nil, nil
	}
	return &migrations[0], nil
}

func (backend *Backend) ReadMigration(pr *core.PullRequest, tableName string) (migration *core.Migration, err error) {
	if pr == nil {
		return migration, fmt.Errorf("ReadMigration: nil pr")
	}
	migrations, err := backend.readMigrations(
		"and migrations.org=? and migrations.repo=? and migrations.pull_request_number=? and migrations.mysql_table=?",
		pr.Org, pr.Repo, pr.Number, tableName,
	)
	if err != nil {
		return migration, err
	}
	if len(migrations) == 0 {
		return nil, nil
	}
	return &migrations[0], nil
}

func (backend *Backend) OwnMigration(migration *core.Migration, token string) (ownedMigration *core.Migration, err error) {
	_, err = backend.db.Exec(`
		update
			migrations
		set
			token=?,
			assigned_timestamp=now(),
			liveness_timestamp=now()
		where
			id=?
			and token=''
		`, token, migration.Id,
	)
	if err != nil {
		return ownedMigration, err
	}
	return backend.ReadMigrationByToken(token)
}

func (backend *Backend) OwnReadyMigration(withStrategy core.MigrationStrategy, token string) (migration *core.Migration, err error) {
	_, err = backend.db.Exec(`
		update
			migrations
		set
			token=?,
			assigned_timestamp=now(),
			liveness_timestamp=now()
		where
			token=''
			and status=?
			and strategy=?
		limit 1
		`, token, core.MigrationStatusReady, withStrategy,
	)
	if err != nil {
		return migration, err
	}
	return backend.ReadMigrationByToken(token)
}

func (backend *Backend) ExpireStaleMigrations(inStatus, toStatus core.MigrationStatus, staleMinutes int) error {
	_, err := backend.db.Exec(`
		update
			migrations
		set
			token='',
			status=?
		where
			token!=''
			and status=?
			and liveness_timestamp < now() - interval ? minute
		`, toStatus, inStatus, staleMinutes,
	)
	return err
}

func (backend *Backend) onLeaderStateChange(newLeaderState int64) error {
	if newLeaderState > 0 {
	} else {
	}
	return nil
}

func (backend *Backend) IsHealthy() bool {
	return atomic.LoadInt64(&backend.healthState) > 0
}

func (backend *Backend) IsLeader() bool {
	return atomic.LoadInt64(&backend.leaderState) > 0
}

func (backend *Backend) GetLeader() string {
	_, leader, _ := backend.ReadLeadership()
	return leader
}

func (backend *Backend) GetStateDescription() string {
	if atomic.LoadInt64(&backend.leaderState) > 0 {
		return "Leader"
	}
	if atomic.LoadInt64(&backend.healthState) > 0 {
		return "Healthy"
	}
	return "Unhealthy"
}

func (backend *Backend) AttemptLeadership() error {
	query := `
    insert ignore into service_election (
        anchor, service_id, last_seen_active
      ) values (
        1, ?, now()
      ) on duplicate key update
			service_id       = if(last_seen_active < now() - interval ? second, values(service_id), service_id),
      last_seen_active = if(service_id = values(service_id), values(last_seen_active), last_seen_active)
  `
	_, err := backend.db.Exec(query, backend.serviceId, electionExpireSeconds)
	return err
}

func (backend *Backend) ForceLeadership() error {
	query := `
    replace into service_election (
        anchor, service_id, last_seen_active
      ) values (
        1, ?, now()
      )
  `
	_, err := backend.db.Exec(query, backend.serviceId)
	return err
}

func (backend *Backend) Reelect() error {
	query := `
    delete from service_election
  `
	_, err := backend.db.Exec(query)
	return err
}

func (backend *Backend) ReadLeadership() (leaderState int64, leader string, err error) {
	query := `
    select
				ifnull(max(service_id), '') as service_id
      from
				service_election
  `
	err = backend.db.Get(&leader, query)
	if leader == backend.serviceId {
		leaderState = 1
	}
	return leaderState, leader, err
}
