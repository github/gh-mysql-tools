package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"

	"github.com/github/skeefree/go/core"
	"github.com/github/skeefree/go/util"

	"github.com/github/mu/kvp"
)

const (
	MigrationOwn          = "migration-own"
	MigrationStarted      = "migration-started"
	MigrationRunning      = "migration-running"
	MigrationNoopComplete = "migration-noop-complete"
	MigrationComplete     = "migration-complete"
	MigrationFailed       = "migration-failed"
)

type GhostMigration struct {
	Cluster      string `json:"cluster"`
	Shard        string `json:"shard"`
	ClusterShard string `json:"cluster_shard"`
	Schema       string `json:"schema"`
	Table        string `json:"table"`
	Alter        string `json:"alter"`
	Suggestion   string `json:"suggestion"`
	Token        string `json:"token"`
	Author       string `json:"author"`
}

func (app *Application) RunCommand(ctx context.Context, command string, token string) error {
	app.Logger.Log(ctx, "cli: running command", kvp.String("command", command))

	switch command {
	case MigrationOwn:
		return app.handleMigrationOwnCommand(ctx, token)
	case MigrationStarted:
		return app.handleMigrationStartedCommand(ctx, token)
	case MigrationRunning:
		return app.handleMigrationRunningCommand(ctx, token)
	case MigrationNoopComplete:
		return app.handleMigrationNoopCompleteCommand(ctx, token)
	case MigrationComplete:
		return app.handleMigrationCompleteCommand(ctx, token)
	case MigrationFailed:
		return app.handleMigrationFailedCommand(ctx, token)
	}
	return nil
}

// pickMigration randomly picks a single migration out of given slice
func (app *Application) pickMigration(ctx context.Context, migrations []core.Migration) (migration *core.Migration) {
	for _, i := range rand.Perm(len(migrations)) {
		return &migrations[i]
	}
	return nil
}

func (app *Application) handleMigrationOwnCommand(ctx context.Context, token string) error {
	if token != "" {
		return fmt.Errorf("migration-own: generates its own token; received token %s", token)
	}
	token, err := util.HostnameToken()
	if err != nil {
		return err
	}
	instance, err := app.sitesAPI.GetInstance(token)
	if err != nil {
		return err
	}

	migration, err := app.backend.ReadMigrationByToken(token)
	if err != nil {
		return err
	}
	if migration != nil {
		return fmt.Errorf("Found existing migration with token %s: %s. Will not own a new migration with this token", token, migration.Canonical)
	}
	// OK, token is valid, no existing migration for this token.

	migrations, err := app.backend.ReadGhostReadyMigrations(instance.Site)
	if err != nil {
		return err
	}
	migration = app.pickMigration(ctx, migrations)
	if migration == nil {
		app.Logger.Log(ctx, "cli: no migration picked")
		return nil
	}

	if migration, err = app.backend.OwnMigration(migration, token); err != nil {
		return err
	}
	if migration == nil {
		app.Logger.Log(ctx, "cli: no migration owned")
		return nil
	}
	// migration is owned!
	app.Logger.Log(ctx, "cli: migration owned", kvp.Any("pr", migration.PR), kvp.String("canonical", migration.Canonical), kvp.String("strategy", string(migration.Strategy)))

	m := &GhostMigration{
		Cluster:      migration.Cluster.Name,
		Shard:        migration.Shard,
		ClusterShard: migration.EvalClusterName(),
		Schema:       migration.Repo.MySQLSchema,
		Table:        migration.TableName,
		Alter:        migration.Alter,
		Suggestion:   migration.Suggestion,
		Token:        migration.Token,
		Author:       migration.PR.Author,
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func (app *Application) handleMigrationStartedCommand(ctx context.Context, token string) error {
	migration, err := app.updateMigrationStatus(ctx, token, core.MigrationStatusRunning, core.MigrationStatusReady, core.MigrationStatusRunning)
	if err != nil {
		return err
	}
	if err := app.commentMigrationStarted(ctx, migration); err != nil {
		return err
	}
	return nil
}

func (app *Application) handleMigrationRunningCommand(ctx context.Context, token string) error {
	_, err := app.updateMigrationStatus(ctx, token, core.MigrationStatusRunning, core.MigrationStatusReady, core.MigrationStatusRunning)
	return err
}

func (app *Application) handleMigrationNoopCompleteCommand(ctx context.Context, token string) error {
	migration, err := app.backend.ReadMigrationByToken(token)
	if err != nil {
		return err
	}
	if migration == nil {
		return fmt.Errorf("Unknown migration with token %s", token)
	}
	if err := app.commentMigrationNoopComplete(ctx, migration); err != nil {
		return err
	}
	return nil
}

func (app *Application) handleMigrationCompleteCommand(ctx context.Context, token string) error {
	migration, err := app.updateMigrationStatus(ctx, token, core.MigrationStatusComplete, core.MigrationStatusRunning)
	if err != nil {
		return err
	}
	if err := app.commentMigrationComplete(ctx, migration); err != nil {
		return err
	}
	return nil
}

func (app *Application) handleMigrationFailedCommand(ctx context.Context, token string) error {
	migration, err := app.updateMigrationStatus(ctx, token, core.MigrationStatusFailed, core.MigrationStatusReady, core.MigrationStatusRunning)
	if err != nil {
		return err
	}
	if err := app.commentMigrationFailed(ctx, migration); err != nil {
		return err
	}
	return nil
}

func (app *Application) updateMigrationStatus(ctx context.Context, token string, toStatus core.MigrationStatus, fromStatuses ...core.MigrationStatus) (migration *core.Migration, err error) {
	migration, err = app.backend.ReadMigrationByToken(token)
	if err != nil {
		return migration, err
	}
	if migration == nil {
		return migration, fmt.Errorf("Unknown migration with token %s", token)
	}
	fromStatusesMap := make(map[core.MigrationStatus]bool)
	for _, s := range fromStatuses {
		fromStatusesMap[s] = true
	}
	if !fromStatusesMap[migration.Status] {
		return migration, fmt.Errorf("Migration status is %s, cannot update to '%s'", migration.Status, toStatus)
	}
	rowsAffected, err := app.backend.UpdateMigrationStatus(migration, migration.Status, toStatus, migration.Strategy)
	if err != nil {
		return migration, err
	}
	if rowsAffected == 0 {
		return migration, fmt.Errorf("No rows affected in updating migration status; token=%s", token)
	}
	return migration, nil
}
