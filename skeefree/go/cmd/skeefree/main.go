package main

import (
	"context"
	"flag"
	"log"
	"os"

	skapp "github.com/github/skeefree/go/app"

	"github.com/github/mu"
	"github.com/github/mu/kvp"
	"github.com/github/mu/logger"
)

// Build version!
var BuildVersion = "skeefree-draft"

func main() {
	selfTest := flag.Bool("self-test", false, "Immediately quit with exit code 0")
	command := flag.String("c", "", "command for CLI execution (empty to run as service)")
	token := flag.String("token", "", "migration token")
	httpAddr := flag.String("http-addr", "", "http address, override HTTP_ADDR environment variable")
	internalAddr := flag.String("internal-addr", "", "internal address, override INTERNAL_ADDR environment variable")
	flag.Parse()

	cliMode := (*command != "")

	if *selfTest {
		os.Exit(0)
	}
	if *httpAddr == "" {
		*httpAddr = os.Getenv("HTTP_ADDR")
	}
	if *httpAddr == "" {
		*httpAddr = ":8080"
	}
	if *internalAddr != "" {
		// Override environment
		os.Setenv("INTERNAL_ADDR", *internalAddr)
	}

	svc, err := mu.New(&mu.Config{
		Name:         "skeefree",
		BuildVersion: BuildVersion,
		HTTPAddr:     *httpAddr,
	})

	if err != nil {
		log.Fatalf("could not create mu service: %+v", err)
	}

	var l *logger.Logger
	if cliMode {
		l = logger.New(&logger.Config{Debug: true, Writer: os.Stderr})
	} else {
		l = svc.Logger()
	}
	app := skapp.NewApplication(l)
	// We want skeefree to not only serve Chatops, but also standard HTTP requests.
	// The apiService defiens the HTTP endpoint this app will serve.
	apiService := skapp.NewApiService(app)
	svc.RouteService(apiService)

	svc.Config.Application = app

	ctx := context.Background()
	l.Log(ctx, "Booting skeefree",
		kvp.String("build_version", BuildVersion),
		kvp.String("http_addr", svc.Config.HTTPAddr),
		kvp.String("grpc_addr", svc.Config.GrpcAddr),
		kvp.String("internal_addr", svc.Config.InternalAddr),
		kvp.String("stats_addr", svc.Config.StatsAddr),
		kvp.Int("pid", os.Getpid()),
	)

	if cliMode {
		if err := app.RunCommand(ctx, *command, *token); err != nil {
			log.Fatalf("error running command %s: %+v", *command, err)
		}
	} else {
		// service mode
		if err := svc.Run(); err != nil {
			log.Fatalf("error running service: %+v", err)
		}
	}
}
