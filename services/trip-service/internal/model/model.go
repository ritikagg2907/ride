package model

import "time"

type Trip struct {
	ID              string     `json:"id"`
	RiderID         string     `json:"rider_id"`
	DriverID        *string    `json:"driver_id,omitempty"`
	RiderEmail      string     `json:"rider_email"`
	RiderPhone      string     `json:"rider_phone"`
	PickupLat       float64    `json:"pickup_lat"`
	PickupLng       float64    `json:"pickup_lng"`
	DropLat         float64    `json:"drop_lat"`
	DropLng         float64    `json:"drop_lng"`
	Fare            *float64   `json:"fare,omitempty"`
	Status          string     `json:"status"`
	PaymentMethod   string     `json:"payment_method"`
	VehicleType     string     `json:"vehicle_type"`
	DurationSeconds *int       `json:"duration_seconds,omitempty"`
	RequestedAt     time.Time  `json:"requested_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Rating struct {
	TripID    string    `json:"trip_id"`
	RaterID   string    `json:"rater_id"`
	RaterRole string    `json:"rater_role"`
	RateeID   string    `json:"ratee_id"`
	RateeRole string    `json:"ratee_role"`
	Score     int       `json:"score"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

type RequestTripReq struct {
	PickupLat     float64 `json:"pickup_lat"`
	PickupLng     float64 `json:"pickup_lng"`
	DropLat       float64 `json:"drop_lat"`
	DropLng       float64 `json:"drop_lng"`
	VehicleType   string  `json:"vehicle_type"`
	PaymentMethod string  `json:"payment_method"`
}

type EstimateReq struct {
	PickupLat   float64 `json:"pickup_lat"`
	PickupLng   float64 `json:"pickup_lng"`
	DropLat     float64 `json:"drop_lat"`
	DropLng     float64 `json:"drop_lng"`
	VehicleType string  `json:"vehicle_type"`
}

type EstimateResp struct {
	EstimatedFare    float64 `json:"estimated_fare"`
	SurgeMultiplier  float64 `json:"surge_multiplier"`
	DistanceKm       float64 `json:"distance_km"`
}

type AssignReq struct {
	DriverID string `json:"driver_id"`
}

type EndTripReq struct {
	DurationSeconds int `json:"duration_seconds"`
}

type RateReq struct {
	RateeID   string `json:"ratee_id"`
	RateeRole string `json:"ratee_role"`
	Score     int    `json:"score"`
	Comment   string `json:"comment"`
}

type CancelReq struct {
	Reason string `json:"reason"`
}

// Rates per vehicle type (per km, per min)
var FareRates = map[string][2]float64{
	"hatchback": {8.0, 1.5},
	"sedan":     {12.0, 2.0},
	"suv":       {18.0, 2.5},
}
