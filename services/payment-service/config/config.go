package config

import "github.com/ride-hailing/shared/pkg/env"

type Config struct {
	Port              string
	DatabaseURL       string
	RedisAddr         string
	RedisPassword     string
	KafkaBrokers      []string
	JWTSecret         string
	RazorpayKeyID     string
	RazorpayKeySecret string
	RazorpayWebhookSecret string
	PaymentProvider   string // "cash" | "razorpay"
}

func Load() Config {
	return Config{
		Port:                  env.Get("PORT", "8085"),
		DatabaseURL:           env.MustGet("PAYMENT_DB_URL"),
		RedisAddr:             env.Get("REDIS_ADDR", "redis:6379"),
		RedisPassword:         env.Get("REDIS_PASSWORD", ""),
		KafkaBrokers:          []string{env.Get("KAFKA_BROKERS", "kafka:9092")},
		JWTSecret:             env.MustGet("JWT_SECRET"),
		RazorpayKeyID:         env.Get("RAZORPAY_KEY_ID", ""),
		RazorpayKeySecret:     env.Get("RAZORPAY_KEY_SECRET", ""),
		RazorpayWebhookSecret: env.Get("RAZORPAY_WEBHOOK_SECRET", ""),
		PaymentProvider:       env.Get("PAYMENT_PROVIDER", "cash"),
	}
}
