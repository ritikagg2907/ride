module github.com/ride-hailing/matching-service

go 1.22

require (
	github.com/google/uuid v1.6.0
	github.com/ride-hailing/shared v0.0.0
	github.com/rs/zerolog v1.32.0
	github.com/segmentio/kafka-go v0.4.47
)

replace github.com/ride-hailing/shared => ../../shared
