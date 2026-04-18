package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/ride-hailing/shared/pkg/jwt"
	"github.com/ride-hailing/shared/pkg/mailer"
	"github.com/ride-hailing/shared/pkg/otp"
	redispkg "github.com/ride-hailing/shared/pkg/redis"
	"github.com/ride-hailing/driver-service/internal/model"
	"github.com/ride-hailing/driver-service/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

const geoKey = "drivers:geo"

var (
	ErrNotFound      = errors.New("driver not found")
	ErrBadCredential = errors.New("invalid credentials")
	ErrOTPInvalid    = errors.New("invalid or expired OTP")
	ErrDuplicate     = errors.New("email or phone already registered")
)

type DriverService struct {
	repo      *repository.DriverRepo
	redis     *redispkg.Client
	jwtSecret string
}

func New(repo *repository.DriverRepo, rc *redispkg.Client, jwtSecret string) *DriverService {
	return &DriverService{repo: repo, redis: rc, jwtSecret: jwtSecret}
}

func (s *DriverService) Register(ctx context.Context, req model.RegisterRequest) (*model.Driver, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	vt := req.VehicleType
	if vt == "" {
		vt = "sedan"
	}
	d := &model.Driver{
		ID: uuid.New().String(), Name: req.Name, Email: req.Email,
		Phone: req.Phone, PasswordHash: string(hash),
		VehicleType: vt, LicensePlate: req.LicensePlate,
	}
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, ErrDuplicate
	}
	return d, nil
}

func (s *DriverService) Login(ctx context.Context, req model.LoginRequest) error {
	d, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return ErrBadCredential
	}
	if err := bcrypt.CompareHashAndPassword([]byte(d.PasswordHash), []byte(req.Password)); err != nil {
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
		To: req.Email, Subject: "Driver Login OTP",
		Body: fmt.Sprintf("<h2>OTP: <b>%s</b></h2><p>Valid 10 min.</p>", code),
	})
}

func (s *DriverService) VerifyLogin(ctx context.Context, req model.VerifyLoginRequest) (*model.AuthResponse, error) {
	ok, err := otp.Verify(ctx, s.redis, req.Email, req.OTP)
	if err != nil || !ok {
		return nil, ErrOTPInvalid
	}
	d, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, ErrNotFound
	}
	return s.issueTokens(d.ID)
}

func (s *DriverService) Refresh(ctx context.Context, refreshToken string) (*model.AuthResponse, error) {
	claims, err := jwt.Validate(s.jwtSecret, refreshToken)
	if err != nil {
		return nil, ErrBadCredential
	}
	return s.issueTokens(claims.UserID)
}

func (s *DriverService) GetByID(ctx context.Context, id string) (*model.Driver, error) {
	d, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	return d, nil
}

func (s *DriverService) UpdateLocation(ctx context.Context, driverID string, lat, lng float64) error {
	// Idempotency: skip if same position seen in last 2s
	idemKey := fmt.Sprintf("loc:idem:%s", driverID)
	lockVal := fmt.Sprintf("%.6f:%.6f", lat, lng)
	set, _ := s.redis.SetNX(ctx, idemKey, lockVal, 2*time.Second)
	if !set {
		return nil // duplicate update, skip
	}

	if err := s.redis.GeoAdd(ctx, geoKey, lng, lat, driverID); err != nil {
		return err
	}
	return s.redis.HSet(ctx, "driver:"+driverID+":meta", map[string]any{
		"status":     "available",
		"lat":        strconv.FormatFloat(lat, 'f', 6, 64),
		"lng":        strconv.FormatFloat(lng, 'f', 6, 64),
		"updated_at": time.Now().Unix(),
	})
}

func (s *DriverService) UpdateStatus(ctx context.Context, driverID, status string) error {
	if err := s.repo.UpdateStatus(ctx, driverID, status); err != nil {
		return err
	}
	_ = s.redis.HSet(ctx, "driver:"+driverID+":meta", map[string]any{"status": status})
	if status == "offline" {
		_ = s.redis.GeoRemove(ctx, geoKey, driverID)
	}
	return nil
}

func (s *DriverService) NearbyDrivers(ctx context.Context, lat, lng, radiusKm float64, vehicleType string) ([]model.NearbyDriver, error) {
	locs, err := s.redis.GeoRadius(ctx, geoKey, lng, lat, radiusKm, 20)
	if err != nil {
		return nil, err
	}
	result := make([]model.NearbyDriver, 0, len(locs))
	for _, loc := range locs {
		meta, _ := s.redis.HGetAll(ctx, "driver:"+loc.Name+":meta")
		if meta["status"] != "available" {
			continue
		}
		if vehicleType != "" && meta["vehicle_type"] != "" && meta["vehicle_type"] != vehicleType {
			continue
		}
		nd := model.NearbyDriver{
			ID: loc.Name, Lat: loc.Latitude, Lng: loc.Longitude,
			DistanceKm: loc.Dist,
		}
		if r, err := strconv.ParseFloat(meta["rating"], 64); err == nil {
			nd.Rating = r
		}
		nd.VehicleType = meta["vehicle_type"]
		result = append(result, nd)
	}
	return result, nil
}

func (s *DriverService) UpdateRating(ctx context.Context, driverID string, score int) error {
	return s.repo.UpdateRating(ctx, driverID, score)
}

// ExpireOfflineDrivers removes drivers not seen in > 60s from geo set.
func (s *DriverService) ExpireOfflineDrivers(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			keys, _ := s.redis.Scan(ctx, "driver:*:meta")
			now := time.Now().Unix()
			for _, key := range keys {
				meta, err := s.redis.HGetAll(ctx, key)
				if err != nil {
					continue
				}
				updatedAt, _ := strconv.ParseInt(meta["updated_at"], 10, 64)
				if now-updatedAt > 60 {
					driverID := key[len("driver:") : len(key)-len(":meta")]
					_ = s.redis.GeoRemove(ctx, geoKey, driverID)
					_ = s.redis.HSet(ctx, key, map[string]any{"status": "offline"})
				}
			}
		}
	}
}

func (s *DriverService) issueTokens(driverID string) (*model.AuthResponse, error) {
	access, err := jwt.Issue(s.jwtSecret, driverID, "driver", 15*time.Minute)
	if err != nil {
		return nil, err
	}
	refresh, err := jwt.Issue(s.jwtSecret, driverID, "driver", 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	return &model.AuthResponse{AccessToken: access, RefreshToken: refresh}, nil
}
