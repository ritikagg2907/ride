package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ride-hailing/shared/pkg/middleware"
	"github.com/ride-hailing/payment-service/internal/service"
)

type Handler struct {
	svc *service.PaymentService
}

func New(svc *service.PaymentService) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes(jwtSecret string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	r.Get("/health/ready", func(w http.ResponseWriter, _ *http.Request) {
		respond(w, 200, map[string]string{"status": "ok"})
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(jwtSecret))
		r.Get("/{id}", h.getPayment)
		r.Get("/history", h.history)
		r.Get("/earnings", h.earnings)
		r.Post("/{id}/confirm-cash", h.confirmCash)
		r.Post("/simulate-success", h.simulateSuccess)
	})

	return r
}

func (h *Handler) getPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respond(w, http.StatusOK, p)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFrom(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit == 0 {
		limit = 20
	}
	payments, err := h.svc.History(r.Context(), claims.UserID, limit, offset)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respond(w, http.StatusOK, payments)
}

func (h *Handler) earnings(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFrom(r.Context())
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "today"
	}
	total, count, err := h.svc.Earnings(r.Context(), claims.UserID, period)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"total_earnings": total,
		"trip_count":     count,
		"period":         period,
	})
}

func (h *Handler) confirmCash(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.ConfirmCash(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) simulateSuccess(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PaymentID         string `json:"payment_id"`
		ProviderPaymentID string `json:"provider_payment_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.svc.MarkCompleted(r.Context(), body.PaymentID, body.ProviderPaymentID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func respond(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
