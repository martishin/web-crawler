package route

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/martishin/web-crawler/internal/handler"
	"github.com/martishin/web-crawler/internal/middleware"
)

func Routes(logger *slog.Logger, api *handler.API) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestIDMiddleware(logger))
	r.Use(middleware.LoggingMiddleware())
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/", api.Index())

	r.Route("/api", func(r chi.Router) {
		r.Post("/update", api.Update())
		r.Get("/users", api.Users())
		r.Get("/posts", api.Posts())
		r.Get("/idf", api.IDF())
	})

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	return r
}
