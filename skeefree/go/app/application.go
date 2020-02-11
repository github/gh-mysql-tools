package app

import (
	"context"
	"fmt"

	skconfig "github.com/github/skeefree/go/config"
	"github.com/github/skeefree/go/db"
	"github.com/github/skeefree/go/gh"
	"github.com/github/skeefree/go/ghapi"

	"github.com/github/go/config"
	"github.com/github/mu"
	"github.com/github/mu/kvp"
	"github.com/github/mu/logger"
)

// Application holds the application configuration and any other
// application global resources.
type Application struct {
	Logger            *logger.Logger
	cfg               *skconfig.Config
	backend           *db.Backend
	ghAPI             *ghapi.GitHubAPI
	sitesAPI          *gh.SitesAPI
	mysqlDiscoveryAPI *gh.MySQLDiscoveryAPI
	scheduler         *Scheduler
	directApplier     *DirectApplier
}

// NewApplication creates a new Application.
func NewApplication(logger *logger.Logger) *Application {
	// Load skeefree-specific config:
	cfg := &skconfig.Config{}
	if err := config.Load(cfg); err != nil {
		panic(fmt.Sprintf("error loading skeefree configuration: %s", err))
	}
	backend, err := db.NewBackend(cfg)
	if err != nil {
		panic(fmt.Sprintf("error creating backend: %s", err))
	}
	ghAPI, err := ghapi.NewGitHubAPI(cfg)
	if err != nil {
		panic(fmt.Sprintf("error creating github client: %s", err))
	}
	sitesAPI, err := gh.NewSitesAPI(cfg)
	if err != nil {
		panic(fmt.Sprintf("error creating sites api client: %s", err))
	}
	mysqlDiscoveryAPI, err := gh.NewMySQLDiscoveryAPI(cfg)
	if err != nil {
		panic(fmt.Sprintf("error creating mysql discovery api client: %s", err))
	}
	scheduler := NewScheduler(cfg, logger, backend, sitesAPI, mysqlDiscoveryAPI)
	directApplier := NewDirectApplier(cfg, logger, backend, mysqlDiscoveryAPI)
	return &Application{
		Logger:            logger,
		cfg:               cfg,
		backend:           backend,
		ghAPI:             ghAPI,
		sitesAPI:          sitesAPI,
		mysqlDiscoveryAPI: mysqlDiscoveryAPI,
		scheduler:         scheduler,
		directApplier:     directApplier,
	}
}

// OnStartUp runs when the application is ready to start. Initialize
// and route or register application services here. This function is
// required.
func (app *Application) OnStartUp(svc *mu.Service) error {
	for _, chatop := range app.chatops() {
		app.Logger.Log(context.Background(), "Registering chatop", kvp.String("name", chatop.name))

		svc.RegisterChatopsCommand(chatop.name, chatop.help, chatop.regexp, chatop.handler)
	}

	go app.backend.ContinuousElections(func(err error) {
		app.Logger.Log(context.Background(), "backend.ContinuousElections", kvp.Error(err))
	})
	go app.ContinuousOperations()

	return nil
}

func (app *Application) IsLeader() bool {
	return app.backend.IsLeader()
}
