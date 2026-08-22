// Package main provides the CLI ingestion tool for Chicago municipal open datasets.
package main

import (
	"flag"
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	dataset := flag.String("dataset", "", "Specific dataset to ingest (cta, crimes, parks, bike_routes, flood, noise, osm_pois, derive)")
	limit := flag.Int("limit", 0, "Limit number of records to ingest (0 = all)")
	all := flag.Bool("all", false, "Ingest all datasets")
	flag.Parse()

	logger.Info("BlockFactor dataset ingestion tool",
		slog.String("dataset", *dataset),
		slog.Int("limit", *limit),
		slog.Bool("all", *all),
	)

	logger.Info("Dataset ingestion pipeline will be implemented in Phase 2")
}
