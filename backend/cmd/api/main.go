package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/health"
	"uptime-app/backend/internal/httpapi"
	"uptime-app/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("API stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	startup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repository, err := store.Open(startup, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer repository.SQL.Close()
	if err := repository.Migrate(startup); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	tokens := &auth.Tokens{Secret: cfg.JWTSecret, Issuer: cfg.JWTIssuer, Audience: cfg.JWTAudience, Now: time.Now}
	service, err := auth.New(repository, tokens)
	if err != nil {
		return err
	}
	avatars, err := auth.NewAvatarStorage(cfg.AvatarDir)
	if err != nil {
		return fmt.Errorf("prepare avatar storage: %w", err)
	}
	authHandler := auth.NewHandler(service, !cfg.LocalHTTP, avatars)
	healthHandler := health.NewHandler(repository.SQL)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.New(authHandler, healthHandler, cfg),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			slog.Error("HTTP shutdown failed", "error", err)
			_ = server.Close()
		}
	}()
	slog.Info("API listening", "address", cfg.HTTPAddr)
	err = server.ListenAndServe()
	stop()
	<-stopped
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
