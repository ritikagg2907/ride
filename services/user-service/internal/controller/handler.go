package controller

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ride-hailing/shared/pkg/middleware"
	"github.com/ride-hailing/user-service/internal/model"
	"github.com/ride-hailing/user-service/internal/service"
)

type Handler struct {
	svc *service.UserService
}

func New(svc *service.UserService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Routes(jwtSecret string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.Post("/verify-login", h.verifyLogin)
	r.Post("/refresh", h.refresh)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(jwtSecret))
		r.Get("/{id}", h.getUser)
	})

	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/health/ready", h.ready)
	return r
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	u, err := h.svc.Register(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	respond(w, http.StatusCreated, u)
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
	respond(w, http.StatusOK, map[string]string{"message": "OTP sent to your email"})
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

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFrom(r.Context())
	id := chi.URLParam(r, "id")
	if claims.UserID != id {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	u, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respond(w, http.StatusOK, u)
}

func (h *Handler) ready(w http.ResponseWriter, _ *http.Request) {
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

func respond(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
