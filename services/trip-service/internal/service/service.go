package service

import (
	"context"
	"errors"
	"math"
	"strconv"

	"github.com/google/uuid"
	kafkapkg "github.com/ride-hailing/shared/pkg/kafka"
	redispkg "github.com/ride-hailing/shared/pkg/redis"
	"github.com/ride-hailing/trip-service/internal/model"
	"github.com/ride-hailing/trip-service/internal/repository"
)

var (
	ErrNotFound    = errors.New("trip not found")
	ErrInvalidState = errors.New("invalid trip state")
)

type TripService struct {
	repo     *repository.TripRepo
	redis    *redispkg.Client
	producer *kafkapkg.Producer
}

func New(repo *repository.TripRepo, rc *redispkg.Client, producer *kafkapkg.Producer) *TripService {
	return &TripService{repo: repo, redis: rc, producer: producer}
}

func (s *TripService) Estimate(ctx context.Context, req model.EstimateReq) model.EstimateResp {
	dist := haversine(req.PickupLat, req.PickupLng, req.DropLat, req.DropLng)
	surge := s.getSurge(ctx)
	rate := fareRate(req.VehicleType)
	est := rate * dist * surge
	return model.EstimateResp{EstimatedFare: est, SurgeMultiplier: surge, DistanceKm: dist}
}

func (s *TripService) RequestTrip(ctx context.Context, riderID, email, phone string, req model.RequestTripReq) (*model.Trip, error) {
	t := &model.Trip{
		ID: uuid.New().String(), RiderID: riderID,
		RiderEmail: email, RiderPhone: phone,
		PickupLat: req.PickupLat, PickupLng: req.PickupLng,
		DropLat: req.DropLat, DropLng: req.DropLng,
		PaymentMethod: req.PaymentMethod, VehicleType: req.VehicleType,
		Status: "REQUESTED",
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	evt := kafkapkg.RideRequestedEvent{
		TripID: t.ID, RiderID: riderID,
		RiderEmail: email, RiderPhone: phone,
		PickupLat: req.PickupLat, PickupLng: req.PickupLng,
		DropLat: req.DropLat, DropLng: req.DropLng,
		VehicleType: req.VehicleType, PaymentMethod: req.PaymentMethod,
	}
	_ = s.producer.Publish(ctx, kafkapkg.TopicRideRequested, t.ID, evt)
	return t, nil
}

func (s *TripService) GetByID(ctx context.Context, id string) (*model.Trip, error) {
	t, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	return t, nil
}

func (s *TripService) Assign(ctx context.Context, tripID, driverID string) error {
	return s.repo.Assign(ctx, tripID, driverID)
}

func (s *TripService) Start(ctx context.Context, tripID, driverID string) error {
	t, err := s.repo.FindByID(ctx, tripID)
	if err != nil {
		return ErrNotFound
	}
	if t.Status != "REQUESTED" && t.Status != "STARTED" {
		return ErrInvalidState
	}
	if err := s.repo.Start(ctx, tripID); err != nil {
		return err
	}
	_ = s.producer.Publish(ctx, kafkapkg.TopicTripStarted, tripID, kafkapkg.TripStartedEvent{
		TripID: tripID, RiderID: t.RiderID, DriverID: driverID,
	})
	return nil
}

func (s *TripService) End(ctx context.Context, tripID string, durationSec int) (*model.Trip, error) {
	t, err := s.repo.FindByID(ctx, tripID)
	if err != nil {
		return nil, ErrNotFound
	}
	if t.Status != "STARTED" {
		return nil, ErrInvalidState
	}
	dist := haversine(t.PickupLat, t.PickupLng, t.DropLat, t.DropLng)
	surge := s.getSurge(ctx)
	rate := fareRate(t.VehicleType)
	durationMin := float64(durationSec) / 60.0
	ratePerMin := fareRatePerMin(t.VehicleType)
	fare := (rate*dist + ratePerMin*durationMin) * surge

	if err := s.repo.End(ctx, tripID, fare, durationSec); err != nil {
		return nil, err
	}
	t.Status = "COMPLETED"
	t.Fare = &fare
	t.DurationSeconds = &durationSec

	driverID := ""
	if t.DriverID != nil {
		driverID = *t.DriverID
	}
	_ = s.producer.Publish(ctx, kafkapkg.TopicTripCompleted, tripID, kafkapkg.TripCompletedEvent{
		TripID: tripID, RiderID: t.RiderID, RiderEmail: t.RiderEmail,
		RiderPhone: t.RiderPhone, DriverID: driverID,
		Fare: fare, PaymentMethod: t.PaymentMethod, DurationSeconds: durationSec,
	})
	return t, nil
}

func (s *TripService) Cancel(ctx context.Context, tripID, riderID, reason string) error {
	t, err := s.repo.FindByID(ctx, tripID)
	if err != nil {
		return ErrNotFound
	}
	if t.Status == "COMPLETED" || t.Status == "CANCELLED" {
		return ErrInvalidState
	}
	if err := s.repo.Cancel(ctx, tripID); err != nil {
		return err
	}
	driverID := ""
	if t.DriverID != nil {
		driverID = *t.DriverID
	}
	_ = s.producer.Publish(ctx, kafkapkg.TopicTripCancelled, tripID, kafkapkg.TripCancelledEvent{
		TripID: tripID, RiderID: riderID, DriverID: driverID, Reason: reason,
	})
	return nil
}

func (s *TripService) Rate(ctx context.Context, tripID, raterID, raterRole string, req model.RateReq) error {
	rt := &model.Rating{
		TripID: tripID, RaterID: raterID, RaterRole: raterRole,
		RateeID: req.RateeID, RateeRole: req.RateeRole,
		Score: req.Score, Comment: req.Comment,
	}
	if err := s.repo.CreateRating(ctx, rt); err != nil {
		return err
	}
	return s.producer.Publish(ctx, kafkapkg.TopicRatingSubmitted, tripID, kafkapkg.RatingSubmittedEvent{
		TripID: tripID, RaterID: raterID, RaterRole: raterRole,
		RateeID: req.RateeID, RateeRole: req.RateeRole,
		Score: req.Score, Comment: req.Comment,
	})
}

func (s *TripService) History(ctx context.Context, userID, role string, limit, offset int) ([]*model.Trip, error) {
	return s.repo.History(ctx, userID, role, limit, offset)
}

func (s *TripService) GetSurge(ctx context.Context) float64 {
	return s.getSurge(ctx)
}

func (s *TripService) SetSurge(ctx context.Context, multiplier float64) error {
	return s.redis.SetRaw(ctx, "surge:multiplier", strconv.FormatFloat(multiplier, 'f', 2, 64), 0)
}

func (s *TripService) getSurge(ctx context.Context) float64 {
	v, err := s.redis.GetRaw(ctx, "surge:multiplier")
	if err != nil {
		return 1.0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 1.0
	}
	return f
}

// haversine returns distance in km between two lat/lng points.
func haversine(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func fareRate(vt string) float64 {
	if r, ok := model.FareRates[vt]; ok {
		return r[0]
	}
	return model.FareRates["sedan"][0]
}

func fareRatePerMin(vt string) float64 {
	if r, ok := model.FareRates[vt]; ok {
		return r[1]
	}
	return model.FareRates["sedan"][1]
}
