package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/martishin/web-crawler/internal/service"
)

type API struct {
	svc *service.Service
}

func NewAPI(svc *service.Service) *API {
	return &API{svc: svc}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func parseUnixParam(r *http.Request, name string, def time.Time) time.Time {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return time.Unix(i, 0).UTC()
}

func (a *API) Index() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"handlers": []string{"POST /api/update", "GET /api/users", "GET /api/posts", "GET /api/idf"},
		})
	}
}

func (a *API) Update() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := a.svc.UpdateToday(r.Context())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "update failed", "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "update successful", "result": res})
	}
}

func (a *API) Users() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		start := parseUnixParam(r, "start", now.Add(-24*time.Hour))
		end := parseUnixParam(r, "end", now)

		users, err := a.svc.ListAuthors(r.Context(), start, end)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users})
	}
}

func (a *API) Posts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		start := parseUnixParam(r, "start", now.Add(-24*time.Hour))
		end := parseUnixParam(r, "end", now)
		user := r.URL.Query().Get("user")
		if user == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "user name not specified"})
			return
		}

		posts, err := a.svc.ListPostsByAuthor(r.Context(), user, start, end)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": user, "posts": posts})
	}
}

func (a *API) IDF() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		start := parseUnixParam(r, "start", now.Add(-24*time.Hour))
		end := parseUnixParam(r, "end", now)
		word := r.URL.Query().Get("word")
		if word == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "word not specified"})
			return
		}

		idf, docs, docsWith, err := a.svc.IDF(r.Context(), word, start, end)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"word":                word,
			"idf":                 idf,
			"documents":           docs,
			"documents_with_word": docsWith,
		})
	}
}
