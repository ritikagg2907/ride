package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ride-hailing/shared/pkg/db"
	kafkapkg "github.com/ride-hailing/shared/pkg/kafka"
	redispkg "github.com/ride-hailing/shared/pkg/redis"
	"github.com/ride-hailing/trip-service/config"
	"github.com/ride-hailing/trip-service/internal/controller"
	migrations "github.com/ride-hailing/trip-service/migrations"
	"github.com/ride-hailing/trip-service/internal/repository"
	"github.com/ride-hailing/trip-service/internal/service"
	"github.com/ride-hailing/trip-service/internal/tracking"
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

	producer := kafkapkg.NewProducer(cfg.KafkaBrokers)
	repo := repository.New(pool)
	svc := service.New(repo, rc, producer)
	hub := tracking.NewHub()
	h := controller.New(svc, hub, cfg.InternalSecret)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: h.Routes(cfg.JWTSecret),
	}

	go func() {
		log.Info().Str("port", cfg.Port).Msg("trip-service starting")
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
	log.Info().Msg("trip-service stopped")
}
