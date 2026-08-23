// Package main provides the CLI ingestion tool for Chicago municipal open datasets.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/cache"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/spatial"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	dataset := flag.String("dataset", "", "Specific dataset to ingest (boundary, cta, crimes, parks, bike_routes, flood, noise, osm_pois, derive)")
	limit := flag.Int("limit", 0, "Limit number of records to ingest (0 = all)")
	all := flag.Bool("all", false, "Ingest all datasets")
	flag.Parse()

	logger.Info("BlockFactor dataset ingestion tool",
		slog.String("dataset", *dataset),
		slog.Int("limit", *limit),
		slog.Bool("all", *all),
	)

	if *dataset == "" && !*all {
		logger.Error("no dataset selected; pass --dataset=<name> or --all")
		os.Exit(1)
	}

	ctx := context.Background()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL environment variable is required")
		os.Exit(1)
	}

	pool, err := spatial.NewPool(ctx, databaseURL)
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	soda := clients.NewSODAClient(os.Getenv("CHICAGO_APP_TOKEN"))
	overpass := clients.NewOverpassClient()
	fema := clients.NewFEMAClient()
	c := cache.New(pool)

	// Every dataset implemented so far ingests its entire source in one
	// pass and doesn't yet respect --limit; each new entry here should
	// revisit that once a dataset actually needs partial ingestion.
	datasets := []struct {
		name string
		run  func(context.Context) error
	}{
		{"boundary", func(ctx context.Context) error { return ingestBoundary(ctx, soda, pool) }},
		{"cta", func(ctx context.Context) error { return ingestCTAStops(ctx, soda, pool) }},
		{"bike_routes", func(ctx context.Context) error { return ingestBikeRoutes(ctx, soda, pool) }},
		{"parks", func(ctx context.Context) error { return ingestParks(ctx, soda, pool) }},
		{"crimes", func(ctx context.Context) error { return ingestCrimes(ctx, soda, pool) }},
		{"osm_pois", func(ctx context.Context) error { return ingestOSMPOIs(ctx, overpass, c, pool) }},
		{"noise", func(ctx context.Context) error { return ingestNoiseSources(ctx, overpass, c, pool) }},
		{"flood_zones", func(ctx context.Context) error { return ingestFloodZones(ctx, fema, c, pool) }},
	}

	ran := false
	for _, d := range datasets {
		if !*all && *dataset != d.name {
			continue
		}
		ran = true

		if *limit > 0 {
			logger.Warn("--limit is ignored for this dataset", slog.String("dataset", d.name))
		}

		logger.Info("ingesting dataset", slog.String("dataset", d.name))
		if err := d.run(ctx); err != nil {
			logger.Error("dataset ingestion failed", slog.String("dataset", d.name), slog.String("error", err.Error()))
			os.Exit(1)
		}
		logger.Info("dataset ingestion complete", slog.String("dataset", d.name))
	}

	if !ran && !*all {
		logger.Error("unknown dataset", slog.String("dataset", *dataset))
		os.Exit(1)
	}
}
