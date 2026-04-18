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
	"github.com/ride-hailing/surge-pricing-service/config"
	"github.com/ride-hailing/surge-pricing-service/internal/controller"
	"github.com/ride-hailing/surge-pricing-service/internal/service"
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
	svc := service.New(rc, producer)
	h := controller.New(svc, cfg.AdminToken)

	// Kafka consumer: RIDE_REQUESTED → increment demand counter
	go func() {
		c := kafkapkg.NewConsumer(cfg.KafkaBrokers, kafkapkg.TopicRideRequested, "surge-demand-tracker")
		defer c.Close()
		c.Consume(ctx, cfg.KafkaBrokers, func(ctx context.Context, msg []byte) error {
			var evt kafkapkg.RideRequestedEvent
			if err := json.Unmarshal(msg, &evt); err != nil {
				return err
			}
			svc.IncrementDemand(ctx, evt.PickupLat, evt.PickupLng)
			return nil
		})
	}()

	// Kafka consumer: SURGE_RECOMPUTE → recompute affected cells
	go func() {
		c := kafkapkg.NewConsumer(cfg.KafkaBrokers, kafkapkg.TopicSurgeRecompute, "surge-recompute")
		defer c.Close()
		c.Consume(ctx, cfg.KafkaBrokers, func(ctx context.Context, msg []byte) error {
			var evt kafkapkg.SurgeRecomputeEvent
			if err := json.Unmarshal(msg, &evt); err != nil {
				return err
			}
			svc.Recompute(ctx, evt.Cells)
			return nil
		})
	}()

	// Periodic ticker fallback every 30s
	go svc.RunTicker(ctx)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: h.Routes(),
	}

	go func() {
		log.Info().Str("port", cfg.Port).Msg("surge-pricing-service starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
	log.Info().Msg("surge-pricing-service stopped")
}
