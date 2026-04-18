package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ride-hailing/shared/pkg/db"
	kafkapkg "github.com/ride-hailing/shared/pkg/kafka"
	redispkg "github.com/ride-hailing/shared/pkg/redis"
	"github.com/ride-hailing/driver-service/config"
	"github.com/ride-hailing/driver-service/internal/controller"
	migrations "github.com/ride-hailing/driver-service/migrations"
	"github.com/ride-hailing/driver-service/internal/repository"
	"github.com/ride-hailing/driver-service/internal/service"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("db connect failed")
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool, migrations.FS, "."); err != nil {
		log.Fatal().Err(err).Msg("migrations failed")
	}

	rc := redispkg.New(cfg.RedisAddr, cfg.RedisPassword, 0)
	if err := rc.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("redis ping failed")
	}

	repo := repository.New(pool)
	svc := service.New(repo, rc, cfg.JWTSecret)

	go svc.ExpireOfflineDrivers(ctx)
	go consumeRatings(ctx, cfg.KafkaBrokers, svc)

	h := controller.New(svc)
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: h.Routes(cfg.JWTSecret, cfg.InternalSecret),
	}

	go func() {
		log.Info().Str("port", cfg.Port).Msg("driver-service starting")
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
	log.Info().Msg("driver-service stopped")
}

func consumeRatings(ctx context.Context, brokers []string, svc *service.DriverService) {
	c := kafkapkg.NewConsumer(brokers, kafkapkg.TopicRatingSubmitted, "driver-service-ratings")
	defer c.Close()
	c.Consume(ctx, brokers, func(ctx context.Context, msg []byte) error {
		var evt kafkapkg.RatingSubmittedEvent
		if err := json.Unmarshal(msg, &evt); err != nil {
			return err
		}
		if evt.RateeRole != "driver" {
			return nil
		}
		return svc.UpdateRating(ctx, evt.RateeID, evt.Score)
	})
}
