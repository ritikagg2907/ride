package config

import "github.com/ride-hailing/shared/pkg/env"

type Config struct {
	Port           string
	RedisAddr      string
	RedisPassword  string
	KafkaBrokers   []string
	InternalSecret string
	TripServiceURL string
}

func Load() Config {
	return Config{
		Port:           env.Get("PORT", "8087"),
		RedisAddr:      env.Get("REDIS_ADDR", "redis:6379"),
		RedisPassword:  env.Get("REDIS_PASSWORD", ""),
		KafkaBrokers:   []string{env.Get("KAFKA_BROKERS", "kafka:9092")},
		InternalSecret: env.MustGet("INTERNAL_SECRET"),
		TripServiceURL: env.Get("TRIP_SERVICE_URL", "http://trip-service:8083"),
	}
}
