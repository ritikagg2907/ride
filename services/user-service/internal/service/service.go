package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/ride-hailing/shared/pkg/jwt"
	"github.com/ride-hailing/shared/pkg/mailer"
	"github.com/ride-hailing/shared/pkg/otp"
	redispkg "github.com/ride-hailing/shared/pkg/redis"
	"github.com/ride-hailing/user-service/internal/model"
	"github.com/ride-hailing/user-service/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrNotFound      = errors.New("user not found")
	ErrBadCredential = errors.New("invalid credentials")
	ErrOTPInvalid    = errors.New("invalid or expired OTP")
	ErrDuplicate     = errors.New("email or phone already registered")
)

type UserService struct {
	repo      *repository.UserRepo
	redis     *redispkg.Client
	jwtSecret string
}

func New(repo *repository.UserRepo, rc *redispkg.Client, jwtSecret string) *UserService {
	return &UserService{repo: repo, redis: rc, jwtSecret: jwtSecret}
}

func (s *UserService) Register(ctx context.Context, req model.RegisterRequest) (*model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &model.User{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: string(hash),
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, ErrDuplicate
	}
	return u, nil
}

func (s *UserService) Login(ctx context.Context, req model.LoginRequest) error {
	u, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return ErrBadCredential
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return ErrBadCredential
	}
	code, err := otp.Generate()
	if err != nil {
		return err
	}
	if err := otp.Store(ctx, s.redis, req.Email, code); err != nil {
		return err
	}
	return mailer.Send(mailer.Mail{
		To:      req.Email,
		Subject: "Your login OTP",
		Body:    "<h2>Your OTP is: <b>" + code + "</b></h2><p>Valid for 10 minutes.</p>",
	})
}

func (s *UserService) VerifyLogin(ctx context.Context, req model.VerifyLoginRequest) (*model.AuthResponse, error) {
	ok, err := otp.Verify(ctx, s.redis, req.Email, req.OTP)
	if err != nil || !ok {
		return nil, ErrOTPInvalid
	}
	u, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, ErrNotFound
	}
	return s.issueTokens(u.ID, "rider")
}

func (s *UserService) Refresh(ctx context.Context, refreshToken string) (*model.AuthResponse, error) {
	claims, err := jwt.Validate(s.jwtSecret, refreshToken)
	if err != nil {
		return nil, ErrBadCredential
	}
	return s.issueTokens(claims.UserID, claims.Role)
}

func (s *UserService) GetByID(ctx context.Context, id string) (*model.User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	return u, nil
}

func (s *UserService) UpdateRating(ctx context.Context, userID string, score int) error {
	return s.repo.UpdateRating(ctx, userID, score)
}

func (s *UserService) issueTokens(userID, role string) (*model.AuthResponse, error) {
	access, err := jwt.Issue(s.jwtSecret, userID, role, 15*time.Minute)
	if err != nil {
		return nil, err
	}
	refresh, err := jwt.Issue(s.jwtSecret, userID, role, 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	return &model.AuthResponse{AccessToken: access, RefreshToken: refresh}, nil
}
