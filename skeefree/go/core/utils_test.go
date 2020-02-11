package core

import (
	"testing"

	test "github.com/openark/golib/tests"
)

func TestEvaluateStrategy(t *testing.T) {
	{
		p := PullRequestMigrationStatement{
			Statement: "CREATE TABLE `t`",
		}
		strategy := EvaluateStrategy(p, true)
		test.S(t).ExpectEquals(strategy, MigrationStrategyDirect)
	}
	{
		p := PullRequestMigrationStatement{
			Statement: "DROP TABLE `t`",
		}
		strategy := EvaluateStrategy(p, true)
		test.S(t).ExpectEquals(strategy, MigrationStrategyDirect)
	}
	{
		p := PullRequestMigrationStatement{
			Statement: "ALTER TABLE `t` DROP KEY `i`",
		}
		strategy := EvaluateStrategy(p, true)
		test.S(t).ExpectEquals(strategy, MigrationStrategyGhost)
	}
	{
		p := PullRequestMigrationStatement{
			Statement: "ALTER TABLE `t` DROP KEY `i`",
		}
		strategy := EvaluateStrategy(p, false)
		test.S(t).ExpectEquals(strategy, MigrationStrategyManual)
	}
	{
		p := PullRequestMigrationStatement{
			Statement: "DROP TABLE `t`",
		}
		strategy := EvaluateStrategy(p, false)
		test.S(t).ExpectEquals(strategy, MigrationStrategyManual)
	}
}
