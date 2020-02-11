package app

import (
	"context"
	"github.com/github/skeefree/go/core"
	"testing"

	test "github.com/openark/golib/tests"
)

func TestFindPRsFulfillableByDirectMigrations(t *testing.T) {
	pr1 := &core.PullRequest{Id: 1}
	pr2 := &core.PullRequest{Id: 2}
	pr3 := &core.PullRequest{Id: 3}
	migrations := []core.Migration{
		{
			PR:       pr1,
			Status:   core.MigrationStatusQueued,
			Strategy: core.MigrationStrategyDirect,
		},
		{
			PR:       pr1,
			Status:   core.MigrationStatusReady,
			Strategy: core.MigrationStrategyDirect,
		},
		{
			PR:       pr1,
			Status:   core.MigrationStatusComplete,
			Strategy: core.MigrationStrategyGhost,
		},
		//
		{
			PR:       pr2,
			Status:   core.MigrationStatusQueued,
			Strategy: core.MigrationStrategyDirect,
		},
		{
			PR:       pr2,
			Status:   core.MigrationStatusQueued,
			Strategy: core.MigrationStrategyGhost,
		},
		{
			PR:       pr2,
			Status:   core.MigrationStatusComplete,
			Strategy: core.MigrationStrategyGhost,
		},
		//
		{
			PR:       pr3,
			Status:   core.MigrationStatusQueued,
			Strategy: core.MigrationStrategyDirect,
		},
		{
			PR:       pr3,
			Status:   core.MigrationStatusQueued,
			Strategy: core.MigrationStrategyDirect,
		},
	}
	scheduler := &Scheduler{}
	prs := scheduler.findPRsFulfillableByDirectMigrations(context.Background(), migrations)
	{
		test.S(t).ExpectEquals(len(prs), 2)
		test.S(t).ExpectEquals(prs[0].Id, int64(1))
		test.S(t).ExpectEquals(prs[1].Id, int64(3))
	}
}
