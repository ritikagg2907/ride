package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ride-hailing/payment-service/internal/model"
)

type PaymentRepo struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *PaymentRepo { return &PaymentRepo{db: db} }

func (r *PaymentRepo) Create(ctx context.Context, p *model.Payment) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO payments (id, trip_id, rider_id, rider_email, rider_phone, driver_id,
		  amount, status, payment_method, provider)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		p.ID, p.TripID, p.RiderID, p.RiderEmail, p.RiderPhone, p.DriverID,
		p.Amount, p.Status, p.PaymentMethod, p.Provider,
	)
	return err
}

func (r *PaymentRepo) FindByID(ctx context.Context, id string) (*model.Payment, error) {
	p := &model.Payment{}
	err := r.db.QueryRow(ctx,
		`SELECT id, trip_id, rider_id, rider_email, driver_id, amount, status,
		        payment_method, provider, provider_order_id, provider_payment_id,
		        failure_reason, attempts_count, created_at, completed_at, updated_at
		 FROM payments WHERE id=$1`, id,
	).Scan(&p.ID, &p.TripID, &p.RiderID, &p.RiderEmail, &p.DriverID, &p.Amount, &p.Status,
		&p.PaymentMethod, &p.Provider, &p.ProviderOrderID, &p.ProviderPaymentID,
		&p.FailureReason, &p.AttemptsCount, &p.CreatedAt, &p.CompletedAt, &p.UpdatedAt,
	)
	return p, err
}

func (r *PaymentRepo) FindByTripID(ctx context.Context, tripID string) (*model.Payment, error) {
	p := &model.Payment{}
	err := r.db.QueryRow(ctx,
		`SELECT id, trip_id, rider_id, driver_id, amount, status, payment_method,
		        provider, attempts_count, created_at, completed_at, updated_at
		 FROM payments WHERE trip_id=$1`, tripID,
	).Scan(&p.ID, &p.TripID, &p.RiderID, &p.DriverID, &p.Amount, &p.Status,
		&p.PaymentMethod, &p.Provider, &p.AttemptsCount, &p.CreatedAt, &p.CompletedAt, &p.UpdatedAt,
	)
	return p, err
}

func (r *PaymentRepo) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE payments SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	return err
}

func (r *PaymentRepo) MarkCompleted(ctx context.Context, id, providerPaymentID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE payments SET status='COMPLETED', provider_payment_id=$2,
		  completed_at=NOW(), updated_at=NOW() WHERE id=$1`,
		id, providerPaymentID,
	)
	return err
}

func (r *PaymentRepo) MarkFailed(ctx context.Context, id, reason string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE payments SET status='FAILED', failure_reason=$2,
		  attempts_count=attempts_count+1, updated_at=NOW() WHERE id=$1`,
		id, reason,
	)
	return err
}

func (r *PaymentRepo) SetProviderOrder(ctx context.Context, id, orderID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE payments SET provider_order_id=$2, status='PROCESSING', updated_at=NOW() WHERE id=$1`,
		id, orderID,
	)
	return err
}

func (r *PaymentRepo) History(ctx context.Context, riderID string, limit, offset int) ([]*model.Payment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, trip_id, amount, status, payment_method, created_at
		 FROM payments WHERE rider_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		riderID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var payments []*model.Payment
	for rows.Next() {
		p := &model.Payment{}
		_ = rows.Scan(&p.ID, &p.TripID, &p.Amount, &p.Status, &p.PaymentMethod, &p.CreatedAt)
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

func (r *PaymentRepo) Earnings(ctx context.Context, driverID string, since time.Time) (float64, int, error) {
	var total float64
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount),0), COUNT(*) FROM payments
		 WHERE driver_id=$1 AND status='COMPLETED' AND completed_at>=$2`,
		driverID, since,
	).Scan(&total, &count)
	return total, count, err
}

func (r *PaymentRepo) StaleProcessing(ctx context.Context, before time.Time) ([]*model.Payment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, provider, provider_order_id FROM payments
		 WHERE status='PROCESSING' AND updated_at < $1`, before,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var payments []*model.Payment
	for rows.Next() {
		p := &model.Payment{}
		_ = rows.Scan(&p.ID, &p.Provider, &p.ProviderOrderID)
		payments = append(payments, p)
	}
	return payments, rows.Err()
}
