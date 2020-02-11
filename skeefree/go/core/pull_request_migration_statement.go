package core

import (
	"strings"
	"time"
)

type MigrationType string

const (
	CreateTableMigrationType   MigrationType = "CreateTableMigrationType"
	DropTableMigrationType     MigrationType = "DropTableMigrationType"
	AlterTableMigrationType    MigrationType = "AlterTableMigrationType"
	AlterDatabaseMigrationType MigrationType = "AlterDatabaseMigrationType"
	UnsupportedMigrationType   MigrationType = "UnsupportedMigrationType"
)

type PullRequestMigrationStatementStatus string

const (
	PullRequestMigrationStatementStatusSuggested PullRequestMigrationStatementStatus = "suggested"
	PullRequestMigrationStatementStatusApproved  PullRequestMigrationStatementStatus = "approved"
	PullRequestMigrationStatementStatusCancelled PullRequestMigrationStatementStatus = "cancelled"
)

type PullRequestMigrationStatement struct {
	Id            int64     `db:"id" json:"id"`
	PullRequestId int64     `db:"pull_requests_id" json:"pull_requests_id"`
	Statement     string    `db:"migration_statement" json:"migration_statement"`
	Status        string    `db:"status" json:"status"`
	TimeAdded     time.Time `db:"added_timestamp" json:"added_timestamp"`
}

func NewPullRequestMigrationStatement() *PullRequestMigrationStatement {
	return &PullRequestMigrationStatement{
		Status: string(PullRequestMigrationStatementStatusSuggested),
	}
}

func (migration *PullRequestMigrationStatement) GetMigrationType() MigrationType {
	if strings.HasPrefix(migration.Statement, "CREATE TABLE") {
		return CreateTableMigrationType
	}
	if strings.HasPrefix(migration.Statement, "DROP TABLE") {
		return DropTableMigrationType
	}
	if strings.HasPrefix(migration.Statement, "RENAME TABLE") && strings.HasSuffix(migration.Statement, "_DRP") {
		return DropTableMigrationType
	}
	if strings.HasPrefix(migration.Statement, "ALTER TABLE") {
		return AlterTableMigrationType
	}
	if strings.HasPrefix(migration.Statement, "ALTER DATABASE") {
		return AlterDatabaseMigrationType
	}
	return UnsupportedMigrationType
}
