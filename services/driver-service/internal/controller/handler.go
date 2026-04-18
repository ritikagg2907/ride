package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ride-hailing/shared/pkg/middleware"
	"github.com/ride-hailing/driver-service/internal/model"
	"github.com/ride-hailing/driver-service/internal/service"
)

type Handler struct {
	svc *service.DriverService
}

func New(svc *service.DriverService) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes(jwtSecret, internalSecret string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.Post("/verify-login", h.verifyLogin)
	r.Post("/refresh", h.refresh)
	r.Get("/nearby", h.nearby)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(jwtSecret))
		r.Get("/{id}", h.getDriver)
		r.Patch("/{id}/location", h.updateLocation)
		r.Patch("/{id}/status", h.updateStatus)
		r.Post("/trips/{tripId}/respond", h.respondTrip)
	})

	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/health/ready", func(w http.ResponseWriter, _ *http.Request) {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return r
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	d, err := h.svc.Register(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	respond(w, http.StatusCreated, d)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.svc.Login(r.Context(), req); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "OTP sent"})
}

func (h *Handler) verifyLogin(w http.ResponseWriter, r *http.Request) {
	var req model.VerifyLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	tokens, err := h.svc.VerifyLogin(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	respond(w, http.StatusOK, tokens)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req model.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	tokens, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	respond(w, http.StatusOK, tokens)
}

func (h *Handler) getDriver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respond(w, http.StatusOK, d)
}

func (h *Handler) updateLocation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	claims := middleware.ClaimsFrom(r.Context())
	if claims.UserID != id {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req model.LocationUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.svc.UpdateLocation(r.Context(), id, req.Lat, req.Lng); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	claims := middleware.ClaimsFrom(r.Context())
	if claims.UserID != id {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req model.StatusUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.svc.UpdateStatus(r.Context(), id, req.Status); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) nearby(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lat, _ := strconv.ParseFloat(q.Get("lat"), 64)
	lng, _ := strconv.ParseFloat(q.Get("lng"), 64)
	radius, _ := strconv.ParseFloat(q.Get("radius_km"), 64)
	if radius == 0 {
		radius = 5
	}
	vehicleType := q.Get("vehicle_type")
	drivers, err := h.svc.NearbyDrivers(r.Context(), lat, lng, radius, vehicleType)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respond(w, http.StatusOK, drivers)
}

func (h *Handler) respondTrip(w http.ResponseWriter, r *http.Request) {
	tripID := chi.URLParam(r, "tripId")
	claims := middleware.ClaimsFrom(r.Context())
	var req model.TripResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Publish driver response to Redis pub/sub for matching-service to pick up.
	// Key: driver:response:{trip_id} = ACCEPT:{driver_id} | DECLINE:{driver_id}
	// matching-service polls this channel.
	_ = r.Context() // used below
	_ = claims.UserID
	_ = tripID
	_ = req.Response
	// Response is handled by matching-service via Redis pub/sub subscription.
	w.WriteHeader(http.StatusNoContent)
}

func respond(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
