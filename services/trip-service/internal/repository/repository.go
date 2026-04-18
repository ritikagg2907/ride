package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ride-hailing/trip-service/internal/model"
)

type TripRepo struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *TripRepo { return &TripRepo{db: db} }

func (r *TripRepo) Create(ctx context.Context, t *model.Trip) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO trips (id, rider_id, rider_email, rider_phone,
		  pickup_lat, pickup_lng, drop_lat, drop_lng,
		  payment_method, vehicle_type, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		t.ID, t.RiderID, t.RiderEmail, t.RiderPhone,
		t.PickupLat, t.PickupLng, t.DropLat, t.DropLng,
		t.PaymentMethod, t.VehicleType, t.Status,
	)
	return err
}

func (r *TripRepo) FindByID(ctx context.Context, id string) (*model.Trip, error) {
	t := &model.Trip{}
	err := r.db.QueryRow(ctx,
		`SELECT id, rider_id, driver_id, rider_email, rider_phone,
		        pickup_lat, pickup_lng, drop_lat, drop_lng, fare, status,
		        payment_method, vehicle_type, duration_seconds,
		        requested_at, started_at, completed_at, created_at
		 FROM trips WHERE id=$1`, id,
	).Scan(&t.ID, &t.RiderID, &t.DriverID, &t.RiderEmail, &t.RiderPhone,
		&t.PickupLat, &t.PickupLng, &t.DropLat, &t.DropLng, &t.Fare, &t.Status,
		&t.PaymentMethod, &t.VehicleType, &t.DurationSeconds,
		&t.RequestedAt, &t.StartedAt, &t.CompletedAt, &t.CreatedAt,
	)
	return t, err
}

func (r *TripRepo) Assign(ctx context.Context, tripID, driverID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE trips SET driver_id=$2, status='STARTED', started_at=NOW() WHERE id=$1`, tripID, driverID)
	return err
}

func (r *TripRepo) Start(ctx context.Context, tripID string) error {
	_, err := r.db.Exec(ctx, `UPDATE trips SET status='STARTED', started_at=NOW() WHERE id=$1`, tripID)
	return err
}

func (r *TripRepo) End(ctx context.Context, tripID string, fare float64, durationSec int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE trips SET status='COMPLETED', fare=$2, duration_seconds=$3, completed_at=NOW() WHERE id=$1`,
		tripID, fare, durationSec)
	return err
}

func (r *TripRepo) Cancel(ctx context.Context, tripID string) error {
	_, err := r.db.Exec(ctx, `UPDATE trips SET status='CANCELLED' WHERE id=$1`, tripID)
	return err
}

func (r *TripRepo) CreateRating(ctx context.Context, rt *model.Rating) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO ratings (trip_id, rater_id, rater_role, ratee_id, ratee_role, score, comment)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (trip_id, rater_id) DO NOTHING`,
		rt.TripID, rt.RaterID, rt.RaterRole, rt.RateeID, rt.RateeRole, rt.Score, rt.Comment,
	)
	return err
}

func (r *TripRepo) History(ctx context.Context, userID, role string, limit, offset int) ([]*model.Trip, error) {
	col := "rider_id"
	if role == "driver" {
		col = "driver_id"
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, rider_id, driver_id, status, fare, vehicle_type,
		        requested_at, started_at, completed_at, created_at
		 FROM trips WHERE `+col+`=$1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trips []*model.Trip
	for rows.Next() {
		t := &model.Trip{}
		if err := rows.Scan(&t.ID, &t.RiderID, &t.DriverID, &t.Status, &t.Fare,
			&t.VehicleType, &t.RequestedAt, &t.StartedAt, &t.CompletedAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		trips = append(trips, t)
	}
	return trips, rows.Err()
}

func (r *TripRepo) SurgeFromRedis(_ context.Context) float64 { return 1.0 } // proxy; actual read in service

// StaleProcessingPayments returns trip IDs where payment might be stuck.
func (r *TripRepo) CompletedUnpaid(ctx context.Context, before time.Time) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id FROM trips WHERE status='COMPLETED' AND completed_at < $1`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
