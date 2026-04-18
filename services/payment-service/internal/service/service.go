package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	kafkapkg "github.com/ride-hailing/shared/pkg/kafka"
	"github.com/ride-hailing/payment-service/internal/model"
	"github.com/ride-hailing/payment-service/internal/repository"
	"github.com/rs/zerolog/log"
	"github.com/sony/gobreaker"
)

type PaymentService struct {
	repo     *repository.PaymentRepo
	producer *kafkapkg.Producer
	cb       *gobreaker.CircuitBreaker
}

func New(repo *repository.PaymentRepo, producer *kafkapkg.Producer) *PaymentService {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "psp",
		MaxRequests: 3,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	})
	return &PaymentService{repo: repo, producer: producer, cb: cb}
}

func (s *PaymentService) CreateFromTrip(ctx context.Context, evt kafkapkg.TripCompletedEvent) (*model.Payment, error) {
	provider := "cash"
	if evt.PaymentMethod == "card" || evt.PaymentMethod == "upi" {
		provider = "razorpay"
	}
	p := &model.Payment{
		ID: uuid.New().String(), TripID: evt.TripID,
		RiderID: evt.RiderID, RiderEmail: evt.RiderEmail, RiderPhone: evt.RiderPhone,
		DriverID: evt.DriverID, Amount: evt.Fare,
		Status: model.StatusPending, PaymentMethod: evt.PaymentMethod, Provider: provider,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	// Async charge
	go s.charge(context.Background(), p)
	return p, nil
}

func (s *PaymentService) charge(ctx context.Context, p *model.Payment) {
	if p.Provider == "cash" {
		_ = s.repo.UpdateStatus(ctx, p.ID, model.StatusAwaitingCashConfirm)
		return
	}
	// Razorpay: use circuit breaker
	_, err := s.cb.Execute(func() (any, error) {
		// In a real integration: call Razorpay API to create order
		// For now, simulate successful order creation
		orderID := "order_" + p.ID[:8]
		return orderID, s.repo.SetProviderOrder(ctx, p.ID, orderID)
	})
	if err != nil {
		log.Error().Err(err).Str("payment_id", p.ID).Msg("PSP charge failed")
		_ = s.repo.MarkFailed(ctx, p.ID, err.Error())
		_ = s.producer.Publish(ctx, kafkapkg.TopicPaymentFailed, p.ID, kafkapkg.PaymentFailedEvent{
			PaymentID: p.ID, TripID: p.TripID,
			RiderID: p.RiderID, RiderEmail: p.RiderEmail, Reason: err.Error(),
		})
	}
}

func (s *PaymentService) ConfirmCash(ctx context.Context, paymentID string) error {
	p, err := s.repo.FindByID(ctx, paymentID)
	if err != nil {
		return err
	}
	if err := s.repo.MarkCompleted(ctx, paymentID, "cash-"+paymentID); err != nil {
		return err
	}
	return s.publishCompleted(ctx, p)
}

func (s *PaymentService) MarkCompleted(ctx context.Context, paymentID, providerPaymentID string) error {
	p, err := s.repo.FindByID(ctx, paymentID)
	if err != nil {
		return err
	}
	if err := s.repo.MarkCompleted(ctx, paymentID, providerPaymentID); err != nil {
		return err
	}
	return s.publishCompleted(ctx, p)
}

func (s *PaymentService) GetByID(ctx context.Context, id string) (*model.Payment, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *PaymentService) History(ctx context.Context, riderID string, limit, offset int) ([]*model.Payment, error) {
	return s.repo.History(ctx, riderID, limit, offset)
}

func (s *PaymentService) Earnings(ctx context.Context, driverID, period string) (float64, int, error) {
	var since time.Time
	now := time.Now()
	switch period {
	case "week":
		since = now.AddDate(0, 0, -7)
	case "month":
		since = now.AddDate(0, -1, 0)
	default: // today
		y, m, d := now.Date()
		since = time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	}
	return s.repo.Earnings(ctx, driverID, since)
}

// Reconcile checks stale PROCESSING payments and resolves them.
func (s *PaymentService) Reconcile(ctx context.Context) {
	before := time.Now().Add(-1 * time.Hour)
	stale, err := s.repo.StaleProcessing(ctx, before)
	if err != nil {
		log.Error().Err(err).Msg("reconcile: query failed")
		return
	}
	for _, p := range stale {
		log.Warn().Str("payment_id", p.ID).Msg("reconcile: stale payment found, marking failed")
		_ = s.repo.MarkFailed(ctx, p.ID, "reconciliation timeout")
	}
}

func (s *PaymentService) RunReconciler(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Reconcile(ctx)
		}
	}
}

func (s *PaymentService) publishCompleted(ctx context.Context, p *model.Payment) error {
	return s.producer.Publish(ctx, kafkapkg.TopicPaymentCompleted, p.ID, kafkapkg.PaymentCompletedEvent{
		PaymentID: p.ID, TripID: p.TripID,
		RiderID: p.RiderID, RiderEmail: p.RiderEmail, DriverID: p.DriverID,
		Amount: p.Amount, CompletedAt: time.Now().Unix(),
	})
}
