// Package main provides the HTTP server entry point for the BlockFactor API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/cache"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/spatial"
)

// defaultGooglePlacesDailyBudgetCalls is used when GOOGLE_PLACES_DAILY_BUDGET_CALLS
// is unset or invalid -- a conservative self-imposed cost guard, not a
// Google-mandated quota (see internal/scoring/factor_third_places.go).
const defaultGooglePlacesDailyBudgetCalls = 500

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

	// Divvy is genuinely Postgres-independent (a live third-party GBFS feed
	// behind an in-process cache, docs/ARCHITECTURE.md §5.4) -- wired here,
	// unconditionally, rather than inside the DATABASE_URL block below.
	divvyInfoURL := os.Getenv("DIVVY_GBFS_STATION_INFO_URL")
	divvyStatusURL := os.Getenv("DIVVY_GBFS_STATION_STATUS_URL")
	if divvyInfoURL != "" && divvyStatusURL != "" {
		divvyClient := clients.NewDivvyClient(divvyInfoURL, divvyStatusURL)
		divvyCache := clients.NewDivvyStationsCache(divvyClient, clients.DivvyCacheTTL, logger)
		serverOpts = append(serverOpts, api.WithDivvyDocksReader(&divvyDocksReaderAdapter{cache: divvyCache}))
	} else {
		logger.Warn("DIVVY_GBFS_STATION_INFO_URL/DIVVY_GBFS_STATION_STATUS_URL not set; Divvy dock layer will be unconfigured")
	}

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
			serverOpts = append(serverOpts, api.WithLayerReader(spatial.NewDBLayerReader(pool)))
			logger.Info("connected to PostGIS database pool")

			placesClient := clients.NewPlacesClient(os.Getenv("GOOGLE_PLACES_API_KEY"), logger)
			dailyBudgetCalls := defaultGooglePlacesDailyBudgetCalls
			if v := os.Getenv("GOOGLE_PLACES_DAILY_BUDGET_CALLS"); v != "" {
				if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
					dailyBudgetCalls = parsed
				} else {
					logger.Warn("invalid GOOGLE_PLACES_DAILY_BUDGET_CALLS, using default",
						slog.String("value", v), slog.Int("default", defaultGooglePlacesDailyBudgetCalls))
				}
			}

			// Shared by both S_third_place scoring and the nearby-places
			// endpoint below (docs/IMPLEMENTATION_PHASES.md Phase 4
			// deliverable 6) -- one singleflight.Group and one daily budget
			// counter, so a concurrent request from each for the same grid
			// cell dedupes into a single Places API call.
			groceryQuerier := scoring.NewDBGroceryQuerier(pool)
			thirdPlaceResolver := scoring.NewThirdPlacePoiResolver(placesClient, cache.New(pool), scoring.NewDBPlacesBudget(pool), dailyBudgetCalls)

			calculators := []scoring.FactorCalculator{
				scoring.NewTransitCalculator(scoring.NewDBTransitQuerier(pool)),
				scoring.NewBikeCalculator(scoring.NewDBBikeQuerier(pool)),
				scoring.NewGroceryCalculator(groceryQuerier),
				scoring.NewGreenCalculator(scoring.NewDBGreenQuerier(pool)),
				scoring.NewNoiseCalculator(scoring.NewDBNoiseQuerier(pool)),
				scoring.NewFloodCalculator(scoring.NewDBFloodQuerier(pool)),
				scoring.NewSafetyCalculator(scoring.NewDBSafetyQuerier(pool)),
				scoring.NewSolarCalculator(),
				scoring.NewThirdPlaceCalculatorFromResolver(thirdPlaceResolver),
				// No live OTP2 client exists yet (Phase 6) -- registered
				// with a nil querier so S_commute still appears in the
				// response, correctly degraded, whenever a destination is
				// supplied, rather than silently vanishing. See
				// ARCHITECTURE.md §6 for why OTP2 is its own phase.
				scoring.NewCommuteCalculator(nil),
			}
			engine := scoring.NewEngine(calculators, logger)
			serverOpts = append(serverOpts, api.WithScoreEngine(&scoreEngineAdapter{engine: engine}))

			placesResolver := scoring.NewNearbyPlacesResolver(thirdPlaceResolver, groceryQuerier)
			serverOpts = append(serverOpts, api.WithPlacesFinder(&placesFinderAdapter{resolver: placesResolver}))
		}
	} else {
		logger.Warn("DATABASE_URL not set; spatial database checks and scoring will be unconfigured")
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
