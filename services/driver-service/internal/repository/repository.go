package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ride-hailing/driver-service/internal/model"
)

type DriverRepo struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *DriverRepo { return &DriverRepo{db: db} }

func (r *DriverRepo) Create(ctx context.Context, d *model.Driver) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO drivers (id, name, email, phone, password_hash, vehicle_type, license_plate)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		d.ID, d.Name, d.Email, d.Phone, d.PasswordHash, d.VehicleType, d.LicensePlate,
	)
	return err
}

func (r *DriverRepo) FindByEmail(ctx context.Context, email string) (*model.Driver, error) {
	d := &model.Driver{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, email, phone, password_hash, vehicle_type, license_plate,
		        status, rating, rating_count, created_at
		 FROM drivers WHERE email=$1`, email,
	).Scan(&d.ID, &d.Name, &d.Email, &d.Phone, &d.PasswordHash,
		&d.VehicleType, &d.LicensePlate, &d.Status, &d.Rating, &d.RatingCount, &d.CreatedAt)
	return d, err
}

func (r *DriverRepo) FindByID(ctx context.Context, id string) (*model.Driver, error) {
	d := &model.Driver{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, email, phone, vehicle_type, license_plate,
		        status, rating, rating_count, created_at
		 FROM drivers WHERE id=$1`, id,
	).Scan(&d.ID, &d.Name, &d.Email, &d.Phone, &d.VehicleType, &d.LicensePlate,
		&d.Status, &d.Rating, &d.RatingCount, &d.CreatedAt)
	return d, err
}

func (r *DriverRepo) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE drivers SET status=$2 WHERE id=$1`, id, status)
	return err
}

func (r *DriverRepo) UpdateRating(ctx context.Context, driverID string, score int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE drivers SET
			rating = (rating * rating_count + $2) / (rating_count + 1),
			rating_count = rating_count + 1
		WHERE id = $1`,
		driverID, score,
	)
	return err
}
