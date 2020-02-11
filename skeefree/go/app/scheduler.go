package app

import (
	"context"

	"github.com/github/skeefree/go/config"
	"github.com/github/skeefree/go/core"
	"github.com/github/skeefree/go/db"
	"github.com/github/skeefree/go/gh"

	"github.com/github/mu/kvp"
	"github.com/github/mu/logger"
)

type prMigrationStatusCount struct {
	pr           *core.PullRequest
	directQueued int
	directReady  int
	complete     int
	total        int
}

func (c *prMigrationStatusCount) prFulfillableByDirectMigrations() bool {
	if c.directQueued == 0 {
		return false
	}
	if c.directQueued+c.directReady+c.complete == c.total {
		return true
	}
	return false
}

// Scheduler is in charge of scheduling next-to-run migrations.
type Scheduler struct {
	cfg               *config.Config
	Logger            *logger.Logger
	backend           *db.Backend
	sitesAPI          *gh.SitesAPI
	mysqlDiscoveryAPI *gh.MySQLDiscoveryAPI
}

// NewScheduler creates a new scheduler object
func NewScheduler(cfg *config.Config, logger *logger.Logger, backend *db.Backend, sitesAPI *gh.SitesAPI, mysqlDiscoveryAPI *gh.MySQLDiscoveryAPI) *Scheduler {
	return &Scheduler{
		cfg:               cfg,
		Logger:            logger,
		backend:           backend,
		sitesAPI:          sitesAPI,
		mysqlDiscoveryAPI: mysqlDiscoveryAPI,
	}
}

func (scheduler *Scheduler) findPRsFulfillableByDirectMigrations(ctx context.Context, migrations []core.Migration) (fulfillablePRs []*core.PullRequest) {
	prDirectCountMap := make(map[int64]*prMigrationStatusCount)
	orderedPrs := []int64{}
	for _, m := range migrations {
		if _, ok := prDirectCountMap[m.PR.Id]; !ok {
			prDirectCountMap[m.PR.Id] = &prMigrationStatusCount{pr: m.PR}
			orderedPrs = append(orderedPrs, m.PR.Id)
		}
		prStatusCount := prDirectCountMap[m.PR.Id]
		prStatusCount.total = prStatusCount.total + 1
		if m.Status == core.MigrationStatusQueued && m.Strategy == core.MigrationStrategyDirect {
			prStatusCount.directQueued = prStatusCount.directQueued + 1
		}
		if m.Status == core.MigrationStatusReady && m.Strategy == core.MigrationStrategyDirect {
			prStatusCount.directReady = prStatusCount.directReady + 1
		}
		if m.Status == core.MigrationStatusComplete {
			prStatusCount.complete = prStatusCount.complete + 1
		}
	}
	for _, id := range orderedPrs {
		if prDirectCountMap[id].prFulfillableByDirectMigrations() {
			fulfillablePRs = append(fulfillablePRs, prDirectCountMap[id].pr)
		}
	}
	return fulfillablePRs
}

func (scheduler *Scheduler) scheduleNextDirectMigrations(ctx context.Context) error {
	strategy := core.MigrationStrategyDirect
	scheduler.Logger.Log(ctx, "scheduler: starting", kvp.String("strategy", string(strategy)))
	// Find a PR (priority descending) where direct migrations will complete the PR
	migrations, err := scheduler.backend.ReadNonCancelledMigrations(nil)
	if err != nil {
		return err
	}
	fulfillablePRs := scheduler.findPRsFulfillableByDirectMigrations(ctx, migrations)
	if len(fulfillablePRs) == 0 {
		// No PRs to be fulfillable by direct migrations
		return nil
	}
	// We pick a single PR (the first)
	pr := fulfillablePRs[0]
	rowsAffected, err := scheduler.backend.UpdatePRMigrationsStatus(pr, core.MigrationStatusQueued, core.MigrationStatusReady, strategy)
	scheduler.Logger.Log(ctx, "scheduler: scheduled", kvp.String("pr", pr.String()), kvp.String("strategy", string(strategy)), kvp.Int("affected", int(rowsAffected)))
	return err
}

// ghostMigrationEligibleToRun checks if given migration has conflicts with any of the rest of given migrations:
// We may only run a single gh-ost migration on same cluster:shard combination (i.e. don't run two gh-ost
// migrations on same topology at the same time).
func (scheduler *Scheduler) ghostMigrationConflicts(ctx context.Context, migration core.Migration, migrations []core.Migration) (conflicts bool) {
	// iterate all migrations; a single conflict means we shouldn't schedule our migration.
	for _, m := range migrations {
		if m.Id == migration.Id {
			// don't compare migration against itself
			continue
		}
		if m.Cluster.Name != migration.Cluster.Name {
			// Different clusters, no conflict
			continue
		}
		if m.Shard != migration.Shard {
			// Different shard, no conflict
			continue
		}
		// Same cluster, same shard. Is there a conflict?
		if m.Status == core.MigrationStatusReady {
			// If it's Ready it just may kick in in the next few seconds. So let's not schedule given migration.
			return true
		}
		if m.Status == core.MigrationStatusRunning {
			// Obviously this cluster/shard is busy, and we shouldn't run another gh-ost migration.
			return true
		}
	}
	// No conflict found
	return false
}

func (scheduler *Scheduler) getMigrationMasterInstance(ctx context.Context, migration *core.Migration) (instance *gh.Instance, err error) {
	migration.Cluster, err = scheduler.mysqlDiscoveryAPI.GetCluster(migration.Cluster.Name)
	if err != nil {
		return instance, err
	}
	topology, err := db.NewTopologyDB(scheduler.cfg, migration)
	if err != nil {
		return instance, err
	}
	var masterHostname string
	if err := topology.Get(&masterHostname, "select @@hostname"); err != nil {
		return instance, err
	}
	scheduler.Logger.Log(ctx, "getMigrationMasterInstance", kvp.String("migration", migration.Canonical), kvp.String("master_hostname", masterHostname))
	return scheduler.sitesAPI.GetInstance(masterHostname)
}

func (scheduler *Scheduler) scheduleNextGhostMigration(ctx context.Context) error {
	strategy := core.MigrationStrategyGhost
	scheduler.Logger.Log(ctx, "scheduler: starting", kvp.String("strategy", string(strategy)))

	migrations, err := scheduler.backend.ReadNonCancelledMigrations(nil)
	if err != nil {
		return err
	}
	// Remember 'migrations' are in priority DESC, id ASC order, which is the perfect order for this scheduler.
	for _, migration := range migrations {
		// The objective is to find a queued gh-ost migration which has no conflict with other (potentially running)
		// migrations
		if migration.Strategy != strategy {
			continue
		}
		// Noteworthy to remember that if the status is "queued" then this implicitly means the migration has
		// been approved by DBInfra.
		if migration.Status != core.MigrationStatusQueued {
			continue
		}
		if scheduler.ghostMigrationConflicts(ctx, migration, migrations) {
			continue
		}
		instance, err := scheduler.getMigrationMasterInstance(ctx, &migration)
		if err != nil {
			return err
		}
		// Looking good! This is the migration we're going to schedule
		if err := scheduler.backend.UpdateMigrationTokenHint(&migration, instance.Site); err != nil {
			return err
		}
		rowsAffected, err := scheduler.backend.UpdateMigrationStatus(&migration, core.MigrationStatusQueued, core.MigrationStatusReady, strategy)
		scheduler.Logger.Log(ctx, "scheduler: scheduled", kvp.String("pr", migration.PR.String()), kvp.String("canonical", migration.Canonical), kvp.String("strategy", string(strategy)), kvp.Int("affected", int(rowsAffected)))
		return err
	}
	// Got here? We've found nothing to schedule.
	return nil
}
