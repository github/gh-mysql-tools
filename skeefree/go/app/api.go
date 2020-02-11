package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/github/mu"
)

// apiService provides HTTP API endpoints to our app.
// Useful endpoints can be, for example:
//   /debug/metrics: golang standard metrics endpoint
//   /_ping: GitHub standard health check endpoint
//   etc.
type apiService struct {
	app *Application
}

func NewApiService(app *Application) *apiService {
	return &apiService{app: app}
}

func (s *apiService) Routes() []mu.Route {
	return []mu.Route{
		mu.Get("/health", s.health),
	}
}

func (s *apiService) ServiceContext(req *http.Request) {
}

func (s *apiService) health(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "skeefree OK - %s\n",
		time.Now().UTC())
}
