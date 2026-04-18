module github.com/ride-hailing/surge-pricing-service

go 1.22

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/ride-hailing/shared v0.0.0
	github.com/rs/zerolog v1.32.0
	github.com/segmentio/kafka-go v0.4.47
)

replace github.com/ride-hailing/shared => ../../shared
