package kafka

// Topic constants — all consumers and producers must use these.
const (
	TopicRideRequested   = "RIDE_REQUESTED"
	TopicRideOffered     = "RIDE_OFFERED"
	TopicRideCancelled   = "RIDE_CANCELLED"
	TopicDriverAssigned  = "DRIVER_ASSIGNED"
	TopicTripStarted     = "TRIP_STARTED"
	TopicTripCompleted   = "TRIP_COMPLETED"
	TopicTripCancelled   = "TRIP_CANCELLED"
	TopicRatingSubmitted = "RATING_SUBMITTED"
	TopicPaymentCompleted = "PAYMENT_COMPLETED"
	TopicPaymentFailed   = "PAYMENT_FAILED"
	TopicSurgeRecompute  = "SURGE_RECOMPUTE"
	TopicAdminFlagChanged = "ADMIN_FLAG_CHANGED"
)

// DLQ returns the dead-letter topic name for a given topic.
func DLQ(topic string) string { return topic + ".DLQ" }

type RideRequestedEvent struct {
	TripID        string  `json:"trip_id"`
	RiderID       string  `json:"rider_id"`
	RiderEmail    string  `json:"rider_email"`
	RiderPhone    string  `json:"rider_phone"`
	PickupLat     float64 `json:"pickup_lat"`
	PickupLng     float64 `json:"pickup_lng"`
	DropLat       float64 `json:"drop_lat"`
	DropLng       float64 `json:"drop_lng"`
	VehicleType   string  `json:"vehicle_type"`
	PaymentMethod string  `json:"payment_method"`
	RetryCount    int     `json:"retry_count"`
}

type RideOfferedEvent struct {
	TripID          string `json:"trip_id"`
	DriverID        string `json:"driver_id"`
	OfferExpiration int64  `json:"offer_expiration"` // unix seconds
}

type DriverAssignedEvent struct {
	TripID    string  `json:"trip_id"`
	DriverID  string  `json:"driver_id"`
	DriverLat float64 `json:"driver_lat"`
	DriverLng float64 `json:"driver_lng"`
}

type RideCancelledEvent struct {
	TripID string `json:"trip_id"`
	Reason string `json:"reason"`
}

type TripStartedEvent struct {
	TripID   string `json:"trip_id"`
	RiderID  string `json:"rider_id"`
	DriverID string `json:"driver_id"`
}

type TripCompletedEvent struct {
	TripID          string  `json:"trip_id"`
	RiderID         string  `json:"rider_id"`
	RiderEmail      string  `json:"rider_email"`
	RiderPhone      string  `json:"rider_phone"`
	DriverID        string  `json:"driver_id"`
	Fare            float64 `json:"fare"`
	PaymentMethod   string  `json:"payment_method"`
	DurationSeconds int     `json:"duration_seconds"`
}

type TripCancelledEvent struct {
	TripID   string `json:"trip_id"`
	RiderID  string `json:"rider_id"`
	DriverID string `json:"driver_id"`
	Reason   string `json:"reason"`
}

type RatingSubmittedEvent struct {
	TripID    string  `json:"trip_id"`
	RaterID   string  `json:"rater_id"`
	RaterRole string  `json:"rater_role"` // rider | driver
	RateeID   string  `json:"ratee_id"`
	RateeRole string  `json:"ratee_role"`
	Score     int     `json:"score"`
	Comment   string  `json:"comment"`
}

type PaymentCompletedEvent struct {
	PaymentID   string  `json:"payment_id"`
	TripID      string  `json:"trip_id"`
	RiderID     string  `json:"rider_id"`
	RiderEmail  string  `json:"rider_email"`
	DriverID    string  `json:"driver_id"`
	Amount      float64 `json:"amount"`
	CompletedAt int64   `json:"completed_at"`
}

type PaymentFailedEvent struct {
	PaymentID  string `json:"payment_id"`
	TripID     string `json:"trip_id"`
	RiderID    string `json:"rider_id"`
	RiderEmail string `json:"rider_email"`
	Reason     string `json:"reason"`
}

type SurgeRecomputeEvent struct {
	Cells []string `json:"cells"` // H3 cell indices to recompute
}

type AdminFlagChangedEvent struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}
