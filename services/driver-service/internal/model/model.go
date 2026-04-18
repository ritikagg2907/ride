package model

import "time"

type Driver struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	PasswordHash string    `json:"-"`
	VehicleType  string    `json:"vehicle_type"`
	LicensePlate string    `json:"license_plate"`
	Status       string    `json:"status"`
	Rating       float64   `json:"rating"`
	RatingCount  int       `json:"rating_count"`
	CreatedAt    time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Password     string `json:"password"`
	VehicleType  string `json:"vehicle_type"`
	LicensePlate string `json:"license_plate"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type VerifyLoginRequest struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type LocationUpdate struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type StatusUpdate struct {
	Status string `json:"status"` // available | busy | offline
}

type NearbyDriver struct {
	ID          string  `json:"id"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	DistanceKm  float64 `json:"distance_km"`
	VehicleType string  `json:"vehicle_type"`
	Rating      float64 `json:"rating"`
}

type TripResponse struct {
	TripID   string `json:"trip_id"`
	Response string `json:"response"` // ACCEPT | DECLINE
}
