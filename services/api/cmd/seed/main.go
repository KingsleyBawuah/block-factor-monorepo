// Package main provides the CLI ingestion tool for Chicago municipal open datasets.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

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

	if *all || *dataset == "boundary" {
		logger.Info("ingesting dataset", slog.String("dataset", "boundary"))
		if err := ingestBoundary(ctx, soda, pool); err != nil {
			logger.Error("boundary ingestion failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		logger.Info("dataset ingestion complete", slog.String("dataset", "boundary"))
	}
}
