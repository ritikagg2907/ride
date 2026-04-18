package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ride-hailing/user-service/internal/model"
)

type UserRepo struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO users (id, name, email, phone, password_hash) VALUES ($1,$2,$3,$4,$5)`,
		u.ID, u.Name, u.Email, u.Phone, u.PasswordHash,
	)
	return err
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, email, phone, password_hash, rating, rating_count, created_at FROM users WHERE email=$1`,
		email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Phone, &u.PasswordHash, &u.Rating, &u.RatingCount, &u.CreatedAt)
	return u, err
}

func (r *UserRepo) FindByID(ctx context.Context, id string) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, email, phone, rating, rating_count, created_at FROM users WHERE id=$1`,
		id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Phone, &u.Rating, &u.RatingCount, &u.CreatedAt)
	return u, err
}

func (r *UserRepo) UpdateRating(ctx context.Context, userID string, score int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET
			rating = (rating * rating_count + $2) / (rating_count + 1),
			rating_count = rating_count + 1
		WHERE id = $1`,
		userID, score,
	)
	return err
}
