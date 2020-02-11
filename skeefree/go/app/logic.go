package app

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/github/mu/kvp"
	"github.com/github/skeefree/go/core"
	"github.com/github/skeefree/go/ghapi"
)

var (
	stateCheckTick    = time.Tick(5 * time.Second)
	prSearchTick      = time.Tick(1 * time.Minute)
	schedulerTick     = time.Tick(1 * time.Minute)
	directApplierTick = time.Tick(1 * time.Minute)
)

const staleMigrationMinutes = 10

var requiresPerShardMigrationMap = map[core.MigrationType]bool{
	core.AlterDatabaseMigrationType: false,
	core.CreateTableMigrationType:   false,
	core.DropTableMigrationType:     true,
	core.AlterTableMigrationType:    true,
}

func (app *Application) StateCheck() error {
	if !app.IsLeader() {
		return nil
	}
	return nil
}

func (app *Application) ContinuousOperations() error {
	ctx := context.Background()
	for {
		select {
		case <-stateCheckTick:
			{
				go func() {
					if err := app.StateCheck(); err != nil {
						app.Logger.Log(ctx, "ContinuousOperations", kvp.Error(err))
					}
				}()
			}
		case <-prSearchTick:
			{
				if app.IsLeader() {
					go func() {
						if err := app.detectApprovedPRs(ctx); err != nil {
							app.Logger.Log(ctx, "detectApprovedPRs", kvp.Error(err))
						}
					}()
					go func() {
						if err := app.probeKnownOpenPRs(ctx); err != nil {
							app.Logger.Log(ctx, "probeKnownOpenPRs", kvp.Error(err))
						}
						// The following should be sequential to the above, otherwise they run into a race condition.
						if err := app.detectAndMarkCompletedPRs(ctx); err != nil {
							app.Logger.Log(ctx, "detectAndMarkCompletedPRs", kvp.Error(err))
						}
					}()
				}
			}
		case <-schedulerTick:
			{
				if app.IsLeader() {
					go func() {
						if err := app.scheduler.scheduleNextDirectMigrations(ctx); err != nil {
							app.Logger.Log(ctx, "scheduleNextDirectMigrations", kvp.Error(err))
						}
					}()
					go func() {
						if err := app.scheduler.scheduleNextGhostMigration(ctx); err != nil {
							app.Logger.Log(ctx, "scheduleNextGhostMigration", kvp.Error(err))
						}
					}()
					go func() {
						if err := app.backend.ExpireStaleMigrations(core.MigrationStatusReady, core.MigrationStatusReady, staleMigrationMinutes); err != nil {
							app.Logger.Log(ctx, "ExpireStaleMigrations", kvp.Error(err))
						}
						if err := app.backend.ExpireStaleMigrations(core.MigrationStatusRunning, core.MigrationStatusFailed, staleMigrationMinutes); err != nil {
							app.Logger.Log(ctx, "ExpireStaleMigrations", kvp.Error(err))
						}
						if err := app.backend.ExpireStaleMigrations(core.MigrationStatusComplete, core.MigrationStatusComplete, staleMigrationMinutes); err != nil {
							// double-ensure cleanup of token
							app.Logger.Log(ctx, "ExpireStaleMigrations", kvp.Error(err))
						}
						if err := app.backend.ExpireStaleMigrations(core.MigrationStatusFailed, core.MigrationStatusFailed, staleMigrationMinutes); err != nil {
							// double-ensure cleanup of token
							app.Logger.Log(ctx, "ExpireStaleMigrations", kvp.Error(err))
						}
					}()

				}
			}
		case <-directApplierTick:
			{
				if app.IsLeader() {
					go func() {
						_, err := app.directApplier.applyNextMigration(ctx,
							func(m *core.Migration) {
								// owned
							}, func(m *core.Migration) {
								// running
								comment := fmt.Sprintf("`skeefree` is running `%s` on `%s/%s` via `%s`", m.Canonical, m.Cluster.Name, m.Repo.MySQLSchema, m.Cluster.RWName)
								if _, err := app.ghAPI.AddPullRequestComment(ctx, m.PR.Org, m.PR.Repo, m.PR.Number, comment); err != nil {
									app.Logger.Log(ctx, "applyNextMigration", kvp.Error(err))
								}
							}, func(m *core.Migration) {
								// complete
								app.commentMigrationComplete(ctx, m)
							}, func(m *core.Migration) {
								// failed
								app.commentMigrationFailed(ctx, m)
							})
						if err != nil {
							app.Logger.Log(ctx, "applyNextMigration", kvp.Error(err))
						}
					}()
				}
			}
		}
	}
	return nil
}

func (app *Application) commentByCLI(ctx context.Context, migration *core.Migration, comment string) error {
	_, err := app.ghAPI.AddPullRequestComment(ctx, migration.PR.Org, migration.PR.Repo, migration.PR.Number, comment)
	if err != nil {
		app.Logger.Log(ctx, "cli", kvp.Error(err))
	}
	return err
}

func (app *Application) commentMigrationStarted(ctx context.Context, migration *core.Migration) error {
	comment := fmt.Sprintf("`%s` migration has been started by `skeefree` :crossed_fingers:", migration.Canonical)
	return app.commentByCLI(ctx, migration, comment)
}

func (app *Application) commentMigrationNoopComplete(ctx context.Context, migration *core.Migration) error {
	comment := fmt.Sprintf("`%s` **noop** migration has been executed by `skeefree` successfully :+1:", migration.Canonical)
	return app.commentByCLI(ctx, migration, comment)
}

func (app *Application) commentMigrationComplete(ctx context.Context, migration *core.Migration) error {
	comment := fmt.Sprintf("`%s` migration has been executed by `skeefree` :tada:", migration.Canonical)
	return app.commentByCLI(ctx, migration, comment)
}

func (app *Application) commentMigrationFailed(ctx context.Context, migration *core.Migration) error {
	comment := fmt.Sprintf("`%s` migration has **FAILED** :cry:", migration.Canonical)
	return app.commentByCLI(ctx, migration, comment)
}

func (app *Application) detectApprovedPRs(ctx context.Context) error {
	repos, err := app.backend.ReadRepositories()
	if err != nil {
		return err
	}
	for _, repo := range repos {
		r := repo
		go func() {
			if err := app.detectRepoApprovedPRs(ctx, &r); err != nil {
				app.Logger.Log(ctx, "detectApprovedPRs", kvp.String("org", r.Org), kvp.String("repo", r.Repo), kvp.Error(err))
			}
		}()
	}
	return nil
}

func (app *Application) detectRepoApprovedPRs(ctx context.Context, repo *core.Repository) error {
	issuesSearchResult, searchString, err := app.ghAPI.SearchSkeemaDiffUndetectedPRs(ctx, repo.OrgRepo())
	app.Logger.Log(ctx, "detectRepoApprovedPRs", kvp.String("searchString", searchString))
	if err != nil {
		return err
	}
	app.Logger.Log(ctx, "detectRepoApprovedPRs", kvp.Int("detected count prs", len(issuesSearchResult.Issues)))
	// PRs in this array are all "undetected": have no "migration:skeefree:detected" label
	for _, pull := range issuesSearchResult.Issues {
		pr := core.NewPullRequestFromRepository(repo, pull.GetNumber())
		// We're going to double check to see that we don't have this PR.
		// Reason for this double-check is that a human may accidentally remove the `migration:skeefree:detected` label.
		if err := app.backend.ReadPR(pr); err == nil {
			if pr.GetStatus() == core.PullRequestStatusComplete {
				app.Logger.Log(ctx, "detectRepoApprovedPRs: silently skipping 'completed' PR", kvp.String("org", pr.Org), kvp.String("repo", pr.Repo), kvp.Int("number", pr.Number))
				continue
			}
		}
		approved, err := app.ghAPI.PullRequestApprovedBySomeone(ctx, pr.Org, pr.Repo, pr.Number)
		if err != nil {
			return err
		}
		if !approved {
			// we only consider PRs that have been approved by _someone_
			continue
		}
		if err := app.probePR(ctx, pr); err != nil {
			return fmt.Errorf("detectRepoApprovedPRs %s/%s/%d: %+v", pr.Org, pr.Repo, pr.Number, err)
		}
	}
	return nil
}

func (app *Application) probePR(ctx context.Context, pr *core.PullRequest) (err error) {
	if pr.RequestedReviewByDBReviewers, err = app.ghAPI.TeamRequestedToReviewPullRequest(ctx, pr.Org, pr.Repo, pr.Number, app.cfg.DBReviewers); err != nil {
		return err
	}
	if pr.ApprovedByDBReviewers, err = app.ghAPI.PullRequestApprovedByTeam(ctx, pr.Org, pr.Repo, pr.Number, app.cfg.DBReviewers); err != nil {
		return err
	}
	if pr.RequestedReviewByDBInfra, err = app.ghAPI.TeamRequestedToReviewPullRequest(ctx, pr.Org, pr.Repo, pr.Number, app.cfg.DBInfra); err != nil {
		return err
	}
	if pr.ApprovedByDBInfra, err = app.ghAPI.PullRequestApprovedByTeam(ctx, pr.Org, pr.Repo, pr.Number, app.cfg.DBInfra); err != nil {
		return err
	}
	pull, err := app.ghAPI.ReadPullRequest(ctx, pr.Org, pr.Repo, pr.Number)
	if err != nil {
		return err
	}
	labels := make(map[string]bool)
	for _, label := range pull.Labels {
		labels[label.GetName()] = true
	}
	pr.LabeledAsDiff = labels[ghapi.MigrationDiffLabel]
	pr.LabeledAsDetected = labels[ghapi.MigrationDetectedLabel]
	pr.LabeledAsQueued = labels[ghapi.MigrationQueuedLabel]
	pr.LabeledForReview = labels[ghapi.MigrationForReviewLabel] || labels[ghapi.MigrationForReviewAlternateLabel]
	if labels[ghapi.MigrationApprovedByDBReviewersLabel] {
		// An alternative mthod to identifying the PR is approved by db-schema-reviewers
		pr.ApprovedByDBReviewers = true
	}
	if labels[ghapi.MigrationApprovedByDBInfraLabel] {
		// An alternative mthod to identifying the PR is approved by databases team
		pr.ApprovedByDBInfra = true
	}

	pr.IsOpen = (pull.GetState() == "open")
	pr.Title = pull.GetTitle()
	pr.Author = pull.GetUser().GetLogin()
	if err := app.backend.SubmitPR(pr); err != nil {
		return err
	}
	return nil
}

func (app *Application) probeKnownOpenPRs(ctx context.Context) error {
	prs, err := app.backend.ReadOpenPRs()
	if err != nil {
		return err
	}
	// For fairness, iterate pull requests in random order
	for _, j := range rand.Perm(len(prs)) {
		pr := prs[j]

		if err := app.probePR(ctx, &pr); err != nil {
			app.Logger.Log(ctx, "probePR", kvp.String("org", pr.Org), kvp.String("repo", pr.Repo), kvp.Int("pr", pr.Number), kvp.Error(err))
		}
		if err := app.analyzeDetectedPR(ctx, pr); err != nil {
			app.Logger.Log(ctx, "probeKnownOpenPRs", kvp.String("org", pr.Org), kvp.String("repo", pr.Repo), kvp.Int("pr", pr.Number), kvp.Error(err))
		}
	}
	return nil
}

func (app *Application) analyzeDetectedPR(ctx context.Context, pr core.PullRequest) error {
	if !pr.IsOpen {
		return nil
	}
	if !pr.LabeledAsDiff {
		return nil
	}
	if !pr.LabeledForReview {
		return nil
	}
	switch pr.GetStatus() {
	case core.PullRequestStatusComplete, core.PullRequestStatusCancelled, core.PullRequestStatusUnknown:
		return nil
	}
	if !pr.LabeledAsDetected {
		if err := app.evaluatePRMigration(ctx, pr); err != nil {
			return err
		}
	}
	// See if we need to request review from DBInfra
	if pr.LabeledAsDetected && pr.LabeledForReview && !pr.RequestedReviewByDBInfra && !pr.ApprovedByDBInfra && (pr.ApprovedByDBReviewers || !reposRequiringDBReviewers[pr.Repo]) {
		if err := app.requestReviewFromTeam(ctx, pr, app.cfg.DBInfra); err != nil {
			return err
		}
	}
	if pr.LabeledAsDetected && pr.ApprovedByDBInfra {
		if err := app.queuePRMigrations(ctx, pr); err != nil {
			return err
		}
	}
	return nil
}

func (app *Application) requestReviewFromTeam(ctx context.Context, pr core.PullRequest, teamSlug string) error {
	newlyRequested, err := app.ghAPI.RequestPullRequestReview(ctx, pr.Org, pr.Repo, pr.Number, teamSlug)
	if err != nil {
		return err
	}
	if !newlyRequested {
		return nil
	}
	return nil
}

func (app *Application) getSkeemaDiffCommentBody(ctx context.Context, pr core.PullRequest) (string, error) {
	comment, err := app.ghAPI.ReadPullRequestSkeemaDiffMagicComment(ctx, pr.Org, pr.Repo, pr.Number)
	if err == nil {
		return comment.GetBody(), nil
	}
	pull, err := app.ghAPI.ReadPullRequest(ctx, pr.Org, pr.Repo, pr.Number)
	if err != nil {
		return "", err
	}
	return pull.GetBody(), nil
}

func (app *Application) evaluatePRMigration(ctx context.Context, pr core.PullRequest) error {
	actionName := getActionName(pr.Repo)
	checkPassing, err := app.ghAPI.IsSkeemaCheckStatusPassingForPullRequest(ctx, pr.Org, pr.Repo, pr.Number, actionName)
	if err != nil {
		return err
	}
	if !checkPassing {
		return fmt.Errorf("evaluatePRMigration %s/%s/%d: action is not in successful state", pr.Org, pr.Repo, pr.Number)
	}

	commentBody, err := app.getSkeemaDiffCommentBody(ctx, pr)
	if err != nil {
		return err
	}
	repo := core.NewRepositoryFromPullRequest(&pr)
	if err := app.backend.ReadRepository(repo); err != nil {
		return err
	}
	repoProductionMappings, err := app.backend.ReadRepositoryMappings(repo)
	if err != nil {
		return err
	}
	skeemaDiffInfo := core.ParseSkeemaDiffStatements(commentBody)
	// See if there's any special mapping. In practice, we see either:
	// - filename mapping
	// - schema name mapping, e,g, for repos which use skeema with multiple schemas
	// I don't see a case where both would be used.
	for _, mapping := range repoProductionMappings {
		app.Logger.Log(ctx, "evaluatePRMigration: mapping", kvp.String("org", pr.Org), kvp.String("repo", pr.Repo), kvp.String("hint", mapping.Hint), kvp.String("cluster", mapping.MySQLCluster), kvp.String("schema", mapping.MySQLSchema), kvp.String("FileName", skeemaDiffInfo.FileName), kvp.String("SchemaName", skeemaDiffInfo.SchemaName))
		if mapping.Hint == skeemaDiffInfo.FileName || mapping.Hint == skeemaDiffInfo.SchemaName {
			app.Logger.Log(ctx, "evaluatePRMigration: mapping match", kvp.String("org", pr.Org), kvp.String("repo", pr.Repo), kvp.String("hint", mapping.Hint), kvp.String("cluster", mapping.MySQLCluster), kvp.String("schema", mapping.MySQLSchema), kvp.String("FileName", skeemaDiffInfo.FileName), kvp.String("SchemaName", skeemaDiffInfo.SchemaName))
			repo.MySQLCluster = mapping.MySQLCluster
			repo.MySQLSchema = mapping.MySQLSchema
		}
	}
	if repo.MySQLCluster == "" {
		return fmt.Errorf("evaluatePRMigration %s/%s/%d: cannot resolve MySQL cluster for this PR", pr.Org, pr.Repo, pr.Number)
	}
	if repo.MySQLSchema == "" {
		return fmt.Errorf("evaluatePRMigration %s/%s/%d: cannot resolve MySQL schema for this PR", pr.Org, pr.Repo, pr.Number)
	}
	if len(skeemaDiffInfo.Statements) == 0 {
		msg := "skeefree expected migration statements but could find none"
		return fmt.Errorf("evaluatePRMigration %s/%s/%d: %s", pr.Org, pr.Repo, pr.Number, msg)
	}

	// This PR is already in our database. But we will re-evaluate it.
	if err := app.backend.ForgetPRStatementsAndMigrations(&pr); err != nil {
		return fmt.Errorf("Error forgetting PR: %+v", err)
	}

	if err := app.backend.SubmitPRStatements(&pr, skeemaDiffInfo.Statements); err != nil {
		return err
	}
	// Data is persisted. Now read it back as _migration_
	prStatements, err := app.backend.ReadPullRequestMigrationStatements(&pr)
	if err != nil {
		return err
	}
	if len(prStatements) == 0 {
		msg := "skeefree expected migration statements in backend DB but could find none"
		_, _ = app.ghAPI.AddPullRequestComment(ctx, pr.Org, pr.Repo, pr.Number, msg)
		return fmt.Errorf("evaluatePRMigration %s/%s/%d: %s", pr.Org, pr.Repo, pr.Number, msg)
	}

	migrations, err := app.evaluateMigrations(ctx, repo, &pr, skeemaDiffInfo.FileName, prStatements)
	if err != nil {
		return err
	}
	// Handle suggestions:
	{
		suggestions := []string{}
		for _, migration := range migrations {
			suggestions = append(suggestions, migration.PrettySuggestion())
		}
		suggestionComment := fmt.Sprintf("Migration instructions for @%s/%s:%s", repo.Org, app.cfg.DBInfra, strings.Join(suggestions, ""))
		if _, err := app.ghAPI.AddPullRequestComment(ctx, pr.Org, pr.Repo, pr.Number, suggestionComment); err != nil {
			return err
		}
	}
	countSubmitted, err := app.backend.SubmitMigrations(migrations)
	if err != nil {
		return err
	}
	app.Logger.Log(ctx, "evaluatePRMigration: submitted migrations", kvp.Int("count", int(countSubmitted)))

	if err := app.ghAPI.AddPullRequestLabel(ctx, pr.Org, pr.Repo, pr.Number, ghapi.MigrationDetectedLabel); err != nil {
		return err
	}
	app.Logger.Log(ctx, "evaluatePRMigration: labeled", kvp.String("org", pr.Org), kvp.String("repo", pr.Repo), kvp.Int("pr", pr.Number), kvp.String("label", ghapi.MigrationDetectedLabel))
	return nil
}

func (app *Application) queuePRMigrations(ctx context.Context, pr core.PullRequest) error {
	app.Logger.Log(ctx, "queuePRMigrations", kvp.String("org", pr.Org), kvp.String("repo", pr.Repo), kvp.Int("pr", pr.Number))
	app.backend.QueuePRMigrations(&pr)
	if !pr.LabeledAsQueued {
		if err := app.ghAPI.AddPullRequestLabel(ctx, pr.Org, pr.Repo, pr.Number, ghapi.MigrationQueuedLabel); err != nil {
			return err
		}
	}
	return nil
}

func (app *Application) evaluateMigrations(ctx context.Context, repo *core.Repository, pr *core.PullRequest, skeemaDiffFile string, prStatements []core.PullRequestMigrationStatement) (migrations [](*core.Migration), err error) {
	app.Logger.Log(ctx, "SearchSkeemaDiffApprovedPRs: suggestion begin", kvp.String("org", repo.Org), kvp.String("repo", repo.Repo), kvp.Int("pr", pr.Number))
	clusterShards, err := app.sitesAPI.MySQLClusterShards(repo.MySQLCluster)
	if err != nil {
		return migrations, err
	}
	cluster, err := app.mysqlDiscoveryAPI.GetCluster(repo.MySQLCluster)
	if err != nil {
		return migrations, err
	}
	app.Logger.Log(ctx, "SearchSkeemaDiffApprovedPRs: suggestion iterating", kvp.String("org", repo.Org), kvp.String("repo", repo.Repo), kvp.Int("pr", pr.Number), kvp.Int("len", len(prStatements)))
	for _, prStatement := range prStatements {
		migrationShards := clusterShards
		if !requiresPerShardMigrationMap[prStatement.GetMigrationType()] {
			migrationShards = []string{""}
		}
		for _, shard := range migrationShards {
			strategy := core.EvaluateStrategy(prStatement, repo.AutoRun)
			migration := core.NewMigration(cluster, shard, repo, pr, prStatement, strategy)
			if err := migration.Evaluate(); err != nil {
				return migrations, err
			}
			migrations = append(migrations, migration)
		}
	}
	return migrations, nil
}

func (app *Application) forgetPR(ctx context.Context, repo *core.Repository, prNumber int, prComment string) (err error) {
	pull, err := app.ghAPI.ReadPullRequest(ctx, repo.Org, repo.Repo, prNumber)
	if err != nil {
		return err
	}
	pr := core.NewPullRequestFromRepository(repo, prNumber)
	if err := app.backend.ReadPR(pr); err != nil {
		return err
	}
	if err := app.backend.ForgetPR(pr); err != nil {
		return fmt.Errorf("Error forgetting PR: %+v", err)
	}

	for _, label := range pull.Labels {
		switch label.GetName() {
		case ghapi.MigrationDetectedLabel, ghapi.MigrationQueuedLabel:
			{
				if err := app.ghAPI.RemovePullRequestLabel(ctx, pr.Org, pr.Repo, pr.Number, label.GetName()); err != nil {
					return fmt.Errorf("Error removing label %s: %+v", label.GetName(), err)
				}
			}
		}
	}

	comment := fmt.Sprintf(prComment)
	if _, err := app.ghAPI.AddPullRequestComment(ctx, repo.Org, repo.Repo, prNumber, comment); err != nil {
		return err
	}
	return nil
}

func (app *Application) detectAndMarkCompletedPRs(ctx context.Context) error {
	prs, err := app.backend.ReadNonCompletedPRsWithCompletedMigrations()
	if err != nil {
		return err
	}
	// For fairness, iterate pull requests in random order
	for _, pr := range prs {
		affected, err := app.backend.UpdatePRStatus(&pr, core.PullRequestStatusComplete)
		if err != nil {
			app.Logger.Log(ctx, "detectAndMarkCompletedPRs", kvp.String("org", pr.Org), kvp.String("repo", pr.Repo), kvp.Int("pr", pr.Number), kvp.Error(err))
			continue
		}
		if affected == 0 {
			continue
		}

		app.Logger.Log(ctx, "detectAndMarkCompletedPRs: updated", kvp.String("org", pr.Org), kvp.String("repo", pr.Repo), kvp.Int("pr", pr.Number), kvp.String("status", string(core.PullRequestStatusComplete)))

		if pr.LabeledAsQueued {
			// PR no longer queued
			if err := app.ghAPI.RemovePullRequestLabel(ctx, pr.Org, pr.Repo, pr.Number, ghapi.MigrationQueuedLabel); err != nil {
				app.Logger.Log(ctx, "detectAndMarkCompletedPRs", kvp.String("org", pr.Org), kvp.String("repo", pr.Repo), kvp.Int("pr", pr.Number), kvp.Error(err))
			}
		}

		comment := fmt.Sprintf("@%s All migrations in this PR are in `%s` status. Please go ahead and follow your standard deploy/merge flow.", pr.Author, core.MigrationStatusComplete)
		if commentAddendum, ok := postCompletePRComments[pr.Repo]; ok {
			comment = fmt.Sprintf("%s\n%s", comment, commentAddendum)
		}
		if _, err := app.ghAPI.AddPullRequestComment(ctx, pr.Org, pr.Repo, pr.Number, comment); err != nil {
			app.Logger.Log(ctx, "detectAndMarkCompletedPRs", kvp.String("org", pr.Org), kvp.String("repo", pr.Repo), kvp.Int("pr", pr.Number), kvp.Error(err))
		}

	}
	return nil
}
