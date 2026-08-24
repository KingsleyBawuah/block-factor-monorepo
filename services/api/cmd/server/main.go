// Package main provides the HTTP server entry point for the BlockFactor API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/spatial"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "4001"
		logger.Info("PORT environment variable not set, defaulting to port", slog.String("port", port))
	}

	userAgent := os.Getenv("NOMINATIM_USER_AGENT")
	geocoder := clients.NewNominatimClient(userAgent, logger)

	var serverOpts []api.ServerOption
	serverOpts = append(serverOpts, api.WithGeocoder(geocoder))

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pool, err := spatial.NewPool(initCtx, databaseURL)
		if err != nil {
			logger.Warn("failed to connect to database pool at startup", slog.String("error", err.Error()))
		} else {
			defer pool.Close()
			serverOpts = append(serverOpts, api.WithBoundaryChecker(spatial.NewDBBoundaryChecker(pool)))
			logger.Info("connected to PostGIS database pool")
		}
	} else {
		logger.Warn("DATABASE_URL not set; spatial database checks will be unconfigured")
	}

	serverImpl := api.NewServer(logger, serverOpts...)
	router := api.NewRouter(serverImpl)

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP server shutdown error", slog.String("error", err.Error()))
		}
		close(idleConnsClosed)
	}()

	logger.Info("starting BlockFactor API server", slog.String("port", port))
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("HTTP server failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	<-idleConnsClosed
	logger.Info("server exited cleanly")
}
