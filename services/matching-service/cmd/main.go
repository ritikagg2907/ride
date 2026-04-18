package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	kafkapkg "github.com/ride-hailing/shared/pkg/kafka"
	redispkg "github.com/ride-hailing/shared/pkg/redis"
	"github.com/ride-hailing/matching-service/config"
	"github.com/ride-hailing/matching-service/internal/service"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rc := redispkg.New(cfg.RedisAddr, cfg.RedisPassword, 0)
	if err := rc.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("redis ping failed")
	}

	producer := kafkapkg.NewProducer(cfg.KafkaBrokers)
	matcher := service.New(rc, producer, cfg.TripServiceURL, cfg.InternalSecret)

	// Minimal HTTP server for health probe only
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: mux}
	go func() {
		log.Info().Str("port", cfg.Port).Msg("matching-service health server starting")
		_ = srv.ListenAndServe()
	}()

	// Main Kafka consumer loop
	consumer := kafkapkg.NewConsumer(cfg.KafkaBrokers, kafkapkg.TopicRideRequested, "matching-service")
	defer consumer.Close()

	log.Info().Msg("matching-service consuming RIDE_REQUESTED")
	go consumer.Consume(ctx, cfg.KafkaBrokers, func(ctx context.Context, msg []byte) error {
		var evt kafkapkg.RideRequestedEvent
		if err := json.Unmarshal(msg, &evt); err != nil {
			return err
		}
		return matcher.HandleRideRequested(ctx, evt)
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
	log.Info().Msg("matching-service stopped")
}
