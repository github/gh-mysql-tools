package core

import (
	"testing"

	test "github.com/openark/golib/tests"
)

func TestStrategy(t *testing.T) {
	{
		m := NewMigration(nil, "", nil, nil, PullRequestMigrationStatement{}, MigrationStrategyManual)
		test.S(t).ExpectEquals(m.Strategy, MigrationStrategyManual)
	}
}

func TestEvalClusterName(t *testing.T) {
	{
		r := &Repository{MySQLCluster: "testing"}
		m := NewMigration(nil, "", r, nil, PullRequestMigrationStatement{}, MigrationStrategyManual)
		test.S(t).ExpectEquals(m.EvalClusterName(), "testing")
	}
	{
		r := &Repository{MySQLCluster: "testing"}
		m := NewMigration(nil, "0080", r, nil, PullRequestMigrationStatement{}, MigrationStrategyManual)
		test.S(t).ExpectEquals(m.EvalClusterName(), "testing-0080")
	}
}
