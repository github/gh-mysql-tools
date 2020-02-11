package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/github/skeefree/go/core"
	"github.com/github/skeefree/go/ghapi"
	"github.com/github/skeefree/go/util"

	crpc "github.com/github/go-crpc"
	"github.com/github/mu/kvp"
)

type showPrResponse struct {
	URI        string                   `json:"pr"`
	Priority   core.PullRequestPriority `json:"priority"`
	IsOpen     bool                     `json:"open"`
	Status     string                   `json:"status"`
	Migrations []string                 `json:"migrations"`
}

// Chatop is a single chatops RPC command.
type Chatop struct {
	name, help, regexp string
	handler            crpc.CommandFunc
}

// chatops returns a list of all the chatops registered for this app.
func (app *Application) chatops() []Chatop {
	return []Chatop{
		{
			name:    "add-repo",
			help:    "add-repo <[org/]repo> <team> | add new repository to be managed by skeefree",
			regexp:  `add-repo (?<repo>[-/_\w]+) (?<team>[-_\w]+)`,
			handler: app.chatopAddRepo,
		},
		{
			name:    "remove-repo",
			help:    "remove-repo <[org/]repo> | DANGER! unlists given repo, skeefree will forget everything about it",
			regexp:  `remove-repo (?<repo>[-/_\w]+)`,
			handler: app.chatopRemoveRepo,
		},
		{
			name:    "update-repo",
			help:    "update-repo <[org/]repo> <team> | update details for given repository",
			regexp:  `update-repo (?<repo>[-/_\w]+) (?<team>[-_\w]+)`,
			handler: app.chatopUpdateRepo,
		},
		{
			name:    "show-repo",
			help:    "show-repo <[org/]repo> | show details of registered repository",
			regexp:  `show-repo (?<repo>[-/_\w]+)`,
			handler: app.chatopShowRepo,
		},
		{
			name:    "which-repos",
			help:    "which-repos | show names of all registered repositories",
			regexp:  `which-repos`,
			handler: app.chatopWhichRepos,
		},
		{
			name:    "show-repos",
			help:    "show-repos | show details of all registered repositories",
			regexp:  `show-repos`,
			handler: app.chatopShowRepos,
		},
		{
			name:    "repo-map",
			help:    "repo-map <[org/]repo> <hint> <cluster> <schema> | map a repo's skeema database to production",
			regexp:  `repo-map (?<repo>[-/_\w]+) (?<hint>[-_.:\w]+) (?<mysql_cluster>[-_\w]+) (?<schema_name>[_\w]+)`,
			handler: app.chatopRepoMap,
		},
		{
			name:    "repo-unmap",
			help:    "repo-unmap <[org/]repo> <hint> | forget a repo-production mapping",
			regexp:  `repo-unmap (?<repo>[-/_\w]+) (?<hint>[-_.:\w]+)`,
			handler: app.chatopRepoUnmap,
		},
		{
			name:    "repo-autorun",
			help:    "repo-autorun <enable|disable> <[org/]repo> | enable or disable auto-migration execution for given repo",
			regexp:  `repo-autorun (?<command>[-_\w]+) (?<repo>[-/_\w]+)`,
			handler: app.chatopRepoAutorun,
		},
		{
			name:    "forget-pr",
			help:    "forget-pr https://github.com/<org>/<repo>/pull/<number> | forget a detected pull request",
			regexp:  `forget-pr (https://github.com/|)(?<org>[-_\w]+)/(?<repo>[-_\w]+)/pull/(?<pr_number>[0-9]+)`,
			handler: app.chatopForgetPR,
		},
		{
			name:    "prioritize-pr",
			help:    "prioritize-pr https://github.com/<org>/<repo>/pull/<number> <urgent|high|normal|low> | set a priority for a pull request",
			regexp:  `prioritize-pr (https://github.com/|)(?<org>[-_\w]+)/(?<repo>[-_\w]+)/pull/(?<pr_number>[0-9]+) (?<priority>[_\w]+)`,
			handler: app.chatopPrioritizePR,
		},
		{
			name:    "show-pr",
			help:    "show-pr https://github.com/<org>/<repo>/pull/<number> | show details about a pull request",
			regexp:  `show-pr (https://github.com/|)(?<org>[-_\w]+)/(?<repo>[-_\w]+)/pull/(?<pr_number>[0-9]+)`,
			handler: app.chatopShowPR,
		},
		{
			name:    "approve-autorun",
			help:    "approve-autorun https://github.com/<org>/<repo>/pull/<number> <table>| Approve auto-execution for a specific migration, provide PR url and tabel name",
			regexp:  `approve-autorun (https://github.com/|)(?<org>[-_\w]+)/(?<repo>[-_\w]+)/pull/(?<pr_number>[0-9]+) (?<table_name>[_\w]+)`,
			handler: app.chatopsApproveAutorun,
		},
		{
			name:    "retry-migration",
			help:    "retry-migration https://github.com/<org>/<repo>/pull/<number> <table>| Retry a failed migration",
			regexp:  `retry-migration (https://github.com/|)(?<org>[-_\w]+)/(?<repo>[-_\w]+)/pull/(?<pr_number>[0-9]+) (?<table_name>[_\w]+)`,
			handler: app.chatopsRetryMigration,
		},
		{
			name:    "mark-complete",
			help:    "mark-complete https://github.com/<org>/<repo>/pull/<number> <table>| Mark a migration as `complete`",
			regexp:  `mark-complete (https://github.com/|)(?<org>[-_\w]+)/(?<repo>[-_\w]+)/pull/(?<pr_number>[0-9]+) (?<table_name>[_\w]+)`,
			handler: app.chatopsMarkComplete,
		},
		{
			name:    "sup",
			help:    "sup | show human friendly status",
			regexp:  `sup`,
			handler: app.chatopSup,
		},
		{
			name:    "status",
			help:    "status | show database-team friendly status",
			regexp:  `status`,
			handler: app.chatopStatus,
		},
	}
}

func (app *Application) marshallResponse(i interface{}) (*crpc.CommandResponse, error) {
	b, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return nil, err
	}
	return &crpc.CommandResponse{
		Result: string(b),
	}, nil
}

func (app *Application) rawResponse(s string) (*crpc.CommandResponse, error) {
	return &crpc.CommandResponse{
		Result: s,
	}, nil
}

// chatopRemoveRepo unlists a repo
func (app *Application) readRepository(params map[string]string) (repository *core.Repository, err error) {
	org, repo, err := util.ParseOrgRepo(params, app.cfg.DefaultOrg)
	if err != nil {
		return nil, err
	}

	repository = &core.Repository{
		Org:  org,
		Repo: repo,
	}

	err = app.backend.ReadRepository(repository)
	return repository, err
}

// chatopRemoveRepo unlists a repo
func (app *Application) readPullRequest(params map[string]string) (pr *core.PullRequest, err error) {
	prNumber, err := strconv.Atoi(params["pr_number"])
	if err != nil {
		return nil, fmt.Errorf("Failed to parse pr_number: %s", err)
	}
	org, repo, err := util.ParseOrgRepo(params, app.cfg.DefaultOrg)
	if err != nil {
		return nil, err
	}

	pr = &core.PullRequest{
		Org:    org,
		Repo:   repo,
		Number: prNumber,
	}

	err = app.backend.ReadPR(pr)
	return pr, err
}

func (app *Application) parseAndValidateRepo(ctx context.Context, r *crpc.CommandRequest) (repository *core.Repository, err error) {

	org, repo, err := util.ParseOrgRepo(r.Params, app.cfg.DefaultOrg)
	if err != nil {
		return nil, err
	}
	if org != app.cfg.DefaultOrg {
		return nil, fmt.Errorf("The only supported org is %s", app.cfg.DefaultOrg)
	}
	if _, err := app.ghAPI.ValidateRepo(ctx, org, repo); err != nil {
		return nil, err
	}

	teamSlug := r.Params["team"]
	if _, err = app.ghAPI.ValidateAdminTeam(ctx, org, repo, teamSlug); err != nil {
		return nil, err
	}

	repository = &core.Repository{
		Org:   org,
		Repo:  repo,
		Owner: teamSlug,
	}
	return repository, nil
}

// chatopAddRepo introduces a new repo to be managed by skeefree
func (app *Application) chatopAddRepo(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `add-repo` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	repository, err := app.parseAndValidateRepo(ctx, r)
	if err != nil {
		return nil, err
	}

	added, err := app.backend.AddRepository(repository)
	if err != nil {
		return nil, err
	}
	if !added {
		return nil, fmt.Errorf("Could not add %s: seems to already exist", repository.OrgRepo())
	}
	return app.chatopShowRepo(ctx, r)
}

// chatopAddRepo introduces a new repo to be managed by skeefree
func (app *Application) chatopUpdateRepo(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `update-repo` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	repo, err := app.readRepository(r.Params)
	if err != nil {
		return nil, err
	}

	updated, err := app.backend.UpdateRepository(repo)
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, fmt.Errorf("Could not update %s: does the repo exist?", repo.OrgRepo())
	}
	return app.chatopShowRepo(ctx, r)
}

// chatopRemoveRepo unlists a repo
func (app *Application) chatopRemoveRepo(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `remove-repo` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	repo, err := app.readRepository(r.Params)
	if err != nil {
		return nil, err
	}
	deleted, err := app.backend.DeleteRepository(repo)
	if err != nil {
		return nil, err
	}
	if !deleted {
		return nil, fmt.Errorf("Could not delete %s: does the repo exist?", repo.OrgRepo())
	}
	return app.marshallResponse(fmt.Sprintf("Repository %s deleted", repo.OrgRepo()))
}

func (app *Application) writeRepo(repo *core.Repository, buf *bytes.Buffer) error {
	mapping, err := app.backend.ReadRepositoryMappings(repo)
	if err != nil {
		return err
	}

	buf.WriteString(fmt.Sprintf("\n\n`%s`: owner: `%s`, autorun: *%t*", repo.OrgRepo(), repo.Owner, repo.AutoRun))
	for _, m := range mapping {
		buf.WriteString(fmt.Sprintf("\n- `%s` maps to `%s/%s`", m.Hint, m.MySQLCluster, m.MySQLSchema))
	}
	if len(mapping) == 0 {
		buf.WriteString(fmt.Sprintf("\n- *No mapping found for this repo*. Map via `.skeefree repo-map ...`"))
	}
	return nil
}

// chatopAddRepo introduces a new repo to be managed by skeefree
func (app *Application) chatopShowRepo(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `show-repo` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	repo, err := app.readRepository(r.Params)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := app.writeRepo(repo, &buf); err != nil {
		return nil, err
	}
	return app.rawResponse(strings.TrimSpace(buf.String()))
}

// chatopShowRepos shows details of all known repos
func (app *Application) chatopShowRepos(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `show-repos` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	repos, err := app.backend.ReadRepositories()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	for _, repo := range repos {
		if err := app.writeRepo(&repo, &buf); err != nil {
			return nil, err
		}
	}
	return app.rawResponse(strings.TrimSpace(buf.String()))
}

// chatopWhichRepos lists names of all known repos
func (app *Application) chatopWhichRepos(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `list-repos` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	repos, err := app.backend.ReadRepositories()
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, repo := range repos {
		names = append(names, repo.OrgRepo())
	}
	return app.marshallResponse(names)
}

// chatopRepoMap: map a repo's skeema database to production
func (app *Application) chatopRepoMap(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `repo-map` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	repo, err := app.readRepository(r.Params)
	if err != nil {
		return nil, err
	}
	m := core.NewRepositoryProductionMappingFromRepo(repo)
	m.Hint = r.Params["hint"]
	m.MySQLSchema = r.Params["schema_name"]
	m.MySQLCluster = r.Params["mysql_cluster"]
	if err := app.backend.WriteRepositoryMapping(m); err != nil {
		return nil, err
	}
	return app.chatopShowRepo(ctx, r)
}

// chatopRepoMap: map a repo's skeema database to production
func (app *Application) chatopRepoUnmap(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `repo-unmap` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	repo, err := app.readRepository(r.Params)
	if err != nil {
		return nil, err
	}
	m := core.NewRepositoryProductionMappingFromRepo(repo)
	m.Hint = r.Params["hint"]
	if err := app.backend.RemoveRepositoryMapping(m); err != nil {
		return nil, err
	}
	return app.chatopShowRepo(ctx, r)
}

// chatopRepoMap: map a repo's skeema database to production
func (app *Application) chatopRepoAutorun(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `repo-autorun` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	command := r.Params["command"]
	autorunEnable := false
	switch command {
	case "enable":
		autorunEnable = true
	case "disable":
		autorunEnable = false
	default:
		return nil, fmt.Errorf("`.repo autorun` command must be either `enable` or `disable`")
	}
	repo, err := app.readRepository(r.Params)
	if err != nil {
		return nil, err
	}
	repo.AutoRun = autorunEnable
	if _, err := app.backend.UpdateRepository(repo); err != nil {
		return nil, err
	}
	return app.chatopShowRepo(ctx, r)
}

// chatopSubmitPRByPath submits a schema change pull request
func (app *Application) chatopForgetPR(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `forget-pr` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	prNumber, err := strconv.Atoi(r.Params["pr_number"])
	if err != nil {
		return nil, fmt.Errorf("Failed to parse pr_number: %s", err)
	}
	repo, err := app.readRepository(r.Params)
	if err != nil {
		return nil, err
	}
	prComment := fmt.Sprintf("This pull request has been forgotten via `.skeefree forget-pr` by @%s in `%s`. `skeefree` will detect it again if approved and has `%s` label", r.User, r.RoomID, ghapi.MigrationDiffLabel)
	if err := app.forgetPR(ctx, repo, prNumber, prComment); err != nil {
		return nil, err
	}
	return app.marshallResponse(prComment)
}

// chatopSubmitPRByPath submits a schema change pull request
func (app *Application) chatopPrioritizePR(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `prioritize-pr` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	pr, err := app.readPullRequest(r.Params)
	if err != nil {
		return nil, err
	}
	priority := core.PullRequestPriorityFromText(r.Params["priority"])

	if _, err := app.backend.UpdatePRPriority(pr, priority); err != nil {
		return nil, err
	}
	return app.chatopShowPR(ctx, r)
}

// chatopShowPR
func (app *Application) chatopShowPR(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `show-pr` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	pr, err := app.readPullRequest(r.Params)
	if err != nil {
		return nil, err
	}
	migrations, err := app.backend.ReadNonCancelledMigrations(pr)
	if err != nil {
		return nil, err
	}
	for i := range migrations {
		m := &migrations[i]
		m.PR = pr
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("\n<%s|%s>: %s\n", pr.URL(), pr.String(), pr.Title))
	buf.WriteString("*pr*: ")
	if pr.IsOpen {
		buf.WriteString("open")
	} else {
		buf.WriteString("closed")
	}
	buf.WriteString(fmt.Sprintf(" *status*: %s", pr.Status))
	buf.WriteString(fmt.Sprintf(" *priority*: %s", pr.Priority.ToText()))
	buf.WriteString("\n")
	for _, m := range migrations {
		buf.WriteString(fmt.Sprintf("- %s\n", m.DescriptionMarkdown()))
	}
	return app.rawResponse(strings.TrimSpace(buf.String()))

}

// chatopsApproveAutorun
func (app *Application) chatopsApproveAutorun(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `approve-autorun` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	pr, err := app.readPullRequest(r.Params)
	if err != nil {
		return nil, err
	}
	tableName := r.Params["table_name"]
	migrations, err := app.backend.ReadNonCancelledMigrations(pr)
	if err != nil {
		return nil, err
	}
	var migration *core.Migration
	for i := range migrations {
		if migrations[i].TableName == tableName {
			migration = &migrations[i]
			break
		}
	}
	if migration == nil {
		return nil, fmt.Errorf("Could not find migration for `%s` in %s", tableName, pr.URL())
	}
	if migration.Strategy != core.MigrationStrategyManual {
		return nil, fmt.Errorf("Strategy for this migration is already `%s`. Can only approve-autorun if the strategy is `%s`", migration.Strategy, core.MigrationStrategyManual)
	}
	strategy := core.EvaluateStrategy(*migration.PRStatement, true) // `true`, because that's the point of this chatops: to override and say "yes, I want autorun even if this repo is not normally automtically-executed"
	if _, err := app.backend.UpdateMigrationStrategy(migration, core.MigrationStrategyManual, strategy); err != nil {
		return nil, err
	}
	response := fmt.Sprintf("new strategy for %s `%s` is `%s`", pr.String(), migration.Canonical, strategy)
	return app.rawResponse(response)
}

// chatopsRetryMigration: given migration in `failed` status, change status to `queued`
func (app *Application) chatopsRetryMigration(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `retry-migration` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	pr, err := app.readPullRequest(r.Params)
	if err != nil {
		return nil, err
	}
	tableName := r.Params["table_name"]
	migration, err := app.backend.ReadMigration(pr, tableName)
	if err != nil {
		return nil, err
	}
	if migration == nil {
		return nil, fmt.Errorf("Could not find migration for `%s` in %s", tableName, pr.URL())
	}
	if migration.Status != core.MigrationStatusFailed {
		return nil, fmt.Errorf("Can only retry migrations in `%s` status. Status for this migration is `%s`.", core.MigrationStatusFailed, migration.Status)
	}
	if _, err := app.backend.UpdateMigrationStatus(migration, migration.Status, core.MigrationStatusQueued, migration.Strategy); err != nil {
		return nil, err
	}
	response := fmt.Sprintf("new status for %s `%s` is `%s`", pr.String(), migration.Canonical, core.MigrationStatusQueued)
	return app.rawResponse(response)
}

// chatopsMarkComplete: mark a migration as `complete`
func (app *Application) chatopsMarkComplete(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `mark-complete` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	pr, err := app.readPullRequest(r.Params)
	if err != nil {
		return nil, err
	}
	tableName := r.Params["table_name"]
	migration, err := app.backend.ReadMigration(pr, tableName)
	if err != nil {
		return nil, err
	}
	if migration == nil {
		return nil, fmt.Errorf("Could not find migration for `%s` in %s", tableName, pr.URL())
	}
	if migration.Status == core.MigrationStatusComplete {
		return nil, fmt.Errorf("Migration is already in `%s` state.", migration.Status)
	}
	if _, err := app.backend.UpdateMigrationStatus(migration, migration.Status, core.MigrationStatusComplete, migration.Strategy); err != nil {
		return nil, err
	}
	response := fmt.Sprintf("new status for %s `%s` is `%s`", pr.String(), migration.Canonical, core.MigrationStatusComplete)
	return app.rawResponse(response)
}

// chatopSup
func (app *Application) chatopSup(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `sup` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	migrations, err := app.backend.ReadNonCancelledMigrations(nil)
	if err != nil {
		return nil, err
	}
	prMigrationsMap, orderedPRIds := core.MapPRMigrations(migrations)

	var buf bytes.Buffer
	var printedPRs = make(map[int64]bool)

	iterateMigrations := func(header string, filter func(pr *core.PullRequest) bool) {
		headerPrinted := false
		for _, prId := range orderedPRIds {
			prMigrations := prMigrationsMap[prId]
			if len(prMigrations) == 0 {
				continue
			}
			pr := prMigrations[0].PR
			if printedPRs[pr.Id] {
				// already printed
				continue
			}
			if !filter(pr) {
				continue
			}
			if !headerPrinted {
				buf.WriteString(fmt.Sprintf("\n\n*%s*\n", header))
				headerPrinted = true
			}

			priorityText := ""
			if pr.Priority != core.PullRequestPriorityNormal {
				priorityText = fmt.Sprintf(" [priority=%s]", pr.Priority.ToText())
			}
			buf.WriteString(fmt.Sprintf("\n<%s|%s>: %s%s\n", pr.URL(), pr.String(), pr.Title, priorityText))
			for _, m := range prMigrations {
				buf.WriteString(fmt.Sprintf("- %s\n", m.DescriptionMarkdown()))
			}
			printedPRs[pr.Id] = true
		}
	}
	iterateMigrations(fmt.Sprintf("NEEDS REVIEW from %s", app.cfg.DBReviewers), func(pr *core.PullRequest) bool {
		return pr.IsOpen && pr.GetStatus() == core.PullRequestStatusDetected && pr.LabeledForReview && !pr.ApprovedByDBReviewers && reposRequiringDBReviewers[pr.Repo]
	})
	iterateMigrations(fmt.Sprintf("NEEDS REVIEW from %s", app.cfg.DBInfra, func(pr *core.PullRequest) bool {
		if reposRequiringDBReviewers[pr.Repo] {
			return pr.IsOpen && pr.GetStatus() == core.PullRequestStatusDetected && pr.LabeledForReview && pr.ApprovedByDBReviewers && !pr.ApprovedByDBInfra
		} else {
			return pr.IsOpen && pr.GetStatus() == core.PullRequestStatusDetected && pr.LabeledForReview && !pr.ApprovedByDBInfra
		}
	})
	iterateMigrations("Approved and queued for migration", func(pr *core.PullRequest) bool {
		return pr.IsOpen && pr.GetStatus() == core.PullRequestStatusQueued && pr.ApprovedByDBInfra
	})
	iterateMigrations("Complete", func(pr *core.PullRequest) bool {
		return pr.IsOpen && pr.GetStatus() == core.PullRequestStatusComplete
	})
	iterateMigrations("Not labeled for review", func(pr *core.PullRequest) bool {
		return !pr.LabeledForReview
	})
	iterateMigrations("Uncategorized", func(pr *core.PullRequest) bool {
		return true
	})

	return app.rawResponse(buf.String())
}

// chatopSup
func (app *Application) chatopStatus(ctx context.Context, r *crpc.CommandRequest) (*crpc.CommandResponse, error) {
	app.Logger.Log(ctx, "received `status` chatop", kvp.String("method", r.Method), kvp.Any("params", r.Params))

	migrations, err := app.backend.ReadNonCancelledMigrations(nil)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	var printedMigrations = make(map[int64]bool)

	iterateMigrations := func(header string, filter func(m *core.Migration) bool) {
		headerPrinted := false
		for _, m := range migrations {
			if !filter(&m) {
				continue
			}
			if !headerPrinted {
				buf.WriteString(fmt.Sprintf("\n\n*%s*", header))
				headerPrinted = true
			}

			priorityText := ""
			if m.PR.Priority != core.PullRequestPriorityNormal {
				priorityText = fmt.Sprintf(" priority=%d", m.PR.Priority)
			}
			runningText := ""
			if m.Status == core.MigrationStatusRunning {
				runningText = fmt.Sprintf("\nRunning on `%s`", m.Token)
			}
			buf.WriteString(fmt.Sprintf("\n\n<%s|%s>: `%s`%s\n`%s/%s`, %s%s", m.PR.URL(), m.PR.String(), m.Canonical, runningText, m.Cluster.Name, m.Repo.MySQLSchema, m.Strategy, priorityText))

			printedMigrations[m.Id] = true
		}
	}
	iterateMigrations("Running", func(m *core.Migration) bool {
		return m.Status == core.MigrationStatusRunning
	})
	iterateMigrations("Failed", func(m *core.Migration) bool {
		return m.Status == core.MigrationStatusFailed
	})
	iterateMigrations("Recently completed", func(m *core.Migration) bool {
		return m.Status == core.MigrationStatusComplete
	})
	iterateMigrations("Ready (soon to be migrated)", func(m *core.Migration) bool {
		return m.Status == core.MigrationStatusReady
	})
	iterateMigrations("Queued", func(m *core.Migration) bool {
		return m.Status == core.MigrationStatusQueued
	})
	iterateMigrations("Proposed", func(m *core.Migration) bool {
		return m.Status == core.MigrationStatusProposed
	})
	iterateMigrations("Uncategorized", func(m *core.Migration) bool {
		return !printedMigrations[m.Id]
	})

	return app.rawResponse(strings.TrimSpace(buf.String()))
}
