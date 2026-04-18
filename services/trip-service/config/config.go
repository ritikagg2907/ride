package config

import "github.com/ride-hailing/shared/pkg/env"

type Config struct {
	Port           string
	DatabaseURL    string
	RedisAddr      string
	RedisPassword  string
	KafkaBrokers   []string
	JWTSecret      string
	InternalSecret string
	AllowedOrigins string
}

func Load() Config {
	return Config{
		Port:           env.Get("PORT", "8083"),
		DatabaseURL:    env.MustGet("TRIP_DB_URL"),
		RedisAddr:      env.Get("REDIS_ADDR", "redis:6379"),
		RedisPassword:  env.Get("REDIS_PASSWORD", ""),
		KafkaBrokers:   []string{env.Get("KAFKA_BROKERS", "kafka:9092")},
		JWTSecret:      env.MustGet("JWT_SECRET"),
		InternalSecret: env.MustGet("INTERNAL_SECRET"),
		AllowedOrigins: env.Get("ALLOWED_ORIGINS", "*"),
	}
}
