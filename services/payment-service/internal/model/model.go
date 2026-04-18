package model

import "time"

type Payment struct {
	ID                 string     `json:"id"`
	TripID             string     `json:"trip_id"`
	RiderID            string     `json:"rider_id"`
	RiderEmail         string     `json:"rider_email"`
	RiderPhone         string     `json:"rider_phone"`
	DriverID           string     `json:"driver_id"`
	Amount             float64    `json:"amount"`
	Status             string     `json:"status"`
	PaymentMethod      string     `json:"payment_method"`
	Provider           string     `json:"provider"`
	ProviderOrderID    *string    `json:"provider_order_id,omitempty"`
	ProviderPaymentID  *string    `json:"provider_payment_id,omitempty"`
	ProviderSignature  *string    `json:"provider_signature,omitempty"`
	FailureReason      *string    `json:"failure_reason,omitempty"`
	AttemptsCount      int        `json:"attempts_count"`
	CreatedAt          time.Time  `json:"created_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// Statuses
const (
	StatusPending             = "PENDING"
	StatusProcessing          = "PROCESSING"
	StatusAwaitingCashConfirm = "AWAITING_CASH_CONFIRM"
	StatusCompleted           = "COMPLETED"
	StatusFailed              = "FAILED"
)
