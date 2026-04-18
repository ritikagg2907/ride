package controller

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ride-hailing/shared/pkg/middleware"
	"github.com/ride-hailing/surge-pricing-service/internal/service"
)

type Handler struct {
	svc        *service.SurgeService
	adminToken string
}

func New(svc *service.SurgeService, adminToken string) *Handler {
	return &Handler{svc: svc, adminToken: adminToken}
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	r.Get("/health/ready", func(w http.ResponseWriter, _ *http.Request) {
		respond(w, 200, map[string]string{"status": "ok"})
	})

	r.Get("/surge/map", h.getMap)

	// lat/lng based queries
	r.Get("/surge", h.getSurge)
	r.Post("/surge/override", h.setOverride)

	return r
}

func (h *Handler) getSurge(w http.ResponseWriter, r *http.Request) {
	lat, _ := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lng, _ := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
	mult := h.svc.GetMultiplier(r.Context(), lat, lng)
	respond(w, http.StatusOK, map[string]float64{"multiplier": mult})
}

func (h *Handler) getMap(w http.ResponseWriter, r *http.Request) {
	cells := h.svc.AllCells(r.Context())
	respond(w, http.StatusOK, cells)
}

func (h *Handler) setOverride(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Admin-Token") != h.adminToken {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		Lat        float64 `json:"lat"`
		Lng        float64 `json:"lng"`
		Multiplier float64 `json:"multiplier"`
		TTLSeconds int     `json:"ttl_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ttl := time.Duration(body.TTLSeconds) * time.Second
	if ttl == 0 {
		ttl = 30 * time.Minute
	}
	h.svc.SetOverride(r.Context(), body.Lat, body.Lng, body.Multiplier, ttl)
	w.WriteHeader(http.StatusNoContent)
}

func respond(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
