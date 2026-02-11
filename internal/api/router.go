package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/AgentMesh-Net/indexer-go/internal/store"
)

// NewRouter creates the HTTP router with all v1 endpoints.
func NewRouter(repo store.Repo, maxBodyBytes int64) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(30 * time.Second))

	h := &handlers{repo: repo, maxBody: maxBodyBytes}

	r.Route("/v1", func(r chi.Router) {
		r.Get("/indexer/info", h.GetInfo)

		r.Post("/tasks", h.PostObject("task"))
		r.Get("/tasks", h.ListObjects("task"))

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
	repo    store.Repo
	maxBody int64
}
