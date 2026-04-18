package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ride-hailing/shared/pkg/middleware"
	"github.com/ride-hailing/trip-service/internal/model"
	"github.com/ride-hailing/trip-service/internal/service"
	"github.com/ride-hailing/trip-service/internal/tracking"
)

type Handler struct {
	svc            *service.TripService
	hub            *tracking.Hub
	internalSecret string
}

func New(svc *service.TripService, hub *tracking.Hub, internalSecret string) *Handler {
	return &Handler{svc: svc, hub: hub, internalSecret: internalSecret}
}

func (h *Handler) Routes(jwtSecret string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	r.Get("/health/ready", func(w http.ResponseWriter, _ *http.Request) { respond(w, 200, map[string]string{"status": "ok"}) })

	// Internal: called by matching-service (protected by X-Internal-Secret)
	r.Patch("/{id}/assign", h.assign)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(jwtSecret))

		r.Post("/estimate", h.estimate)
		r.Post("/request", h.requestTrip)
		r.Get("/{id}", h.getTrip)
		r.Patch("/{id}/start", h.start)
		r.Patch("/{id}/end", h.end)
		r.Patch("/{id}/cancel", h.cancel)
		r.Post("/{id}/rate", h.rate)
		r.Get("/history", h.history)
		r.Get("/surge", h.getSurge)
		r.Patch("/surge", h.setSurge)
	})

	// WebSocket — driver pushes location, rider receives it
	r.Get("/ws/{tripId}", func(w http.ResponseWriter, r *http.Request) {
		tripID := chi.URLParam(r, "tripId")
		h.hub.ServeWS(w, r, tripID)
	})

	return r
}

func (h *Handler) estimate(w http.ResponseWriter, r *http.Request) {
	var req model.EstimateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	resp := h.svc.Estimate(r.Context(), req)
	respond(w, http.StatusOK, resp)
}

func (h *Handler) requestTrip(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFrom(r.Context())
	var req model.RequestTripReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// email/phone would come from user-service; for now use placeholder from token
	t, err := h.svc.RequestTrip(r.Context(), claims.UserID, "", "", req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respond(w, http.StatusCreated, t)
}

func (h *Handler) getTrip(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respond(w, http.StatusOK, t)
}

func (h *Handler) assign(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Internal-Secret") != h.internalSecret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	tripID := chi.URLParam(r, "id")
	var req model.AssignReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.svc.Assign(r.Context(), tripID, req.DriverID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFrom(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.svc.Start(r.Context(), id, claims.UserID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) end(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.EndTripReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	t, err := h.svc.End(r.Context(), id, req.DurationSeconds)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, http.StatusOK, t)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFrom(r.Context())
	id := chi.URLParam(r, "id")
	var req model.CancelReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.svc.Cancel(r.Context(), id, claims.UserID, req.Reason); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) rate(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFrom(r.Context())
	id := chi.URLParam(r, "id")
	var req model.RateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.svc.Rate(r.Context(), id, claims.UserID, claims.Role, req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFrom(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit == 0 {
		limit = 20
	}
	trips, err := h.svc.History(r.Context(), claims.UserID, claims.Role, limit, offset)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respond(w, http.StatusOK, trips)
}

func (h *Handler) getSurge(w http.ResponseWriter, r *http.Request) {
	m := h.svc.GetSurge(r.Context())
	respond(w, http.StatusOK, map[string]float64{"multiplier": m})
}

func (h *Handler) setSurge(w http.ResponseWriter, r *http.Request) {
	var body map[string]float64
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.svc.SetSurge(r.Context(), body["multiplier"]); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func respond(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
