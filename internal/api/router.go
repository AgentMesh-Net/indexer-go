package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/AgentMesh-Net/indexer-go/internal/config"
	"github.com/AgentMesh-Net/indexer-go/internal/store"
)

// NewRouter creates the HTTP router with all v1 endpoints.
func NewRouter(repo store.Repo, taskRepo store.TaskRepo, cfg config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(30 * time.Second))

	h := &handlers{repo: repo, taskRepo: taskRepo, maxBody: cfg.MaxBodyBytes, cfg: cfg}

	// Phase 5: structured task endpoints
	r.Get("/v1/health", h.GetHealth)
	r.Get("/v1/meta", h.GetMeta)
	r.Post("/v1/tasks", h.PostTask)
	r.Get("/v1/tasks", h.ListTasks)
	r.Get("/v1/tasks/{taskID}", h.GetTask)
	r.Post("/v1/tasks/{taskID}/accept", h.PostTaskAccept)

	// Legacy envelope endpoints
	r.Route("/v1", func(r chi.Router) {
		r.Get("/indexer/info", h.GetInfo)

		r.Post("/bids", h.PostObject("bid"))
		r.Get("/bids", h.ListObjects("bid"))

		r.Post("/accepts", h.PostAccept)
		r.Get("/accepts", h.ListObjects("accept"))

		r.Post("/artifacts", h.PostObject("artifact"))
		r.Get("/artifacts", h.ListObjects("artifact"))
	})

	return r
}

type handlers struct {
	repo     store.Repo
	taskRepo store.TaskRepo
	maxBody  int64
	cfg      config.Config
}
