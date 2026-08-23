package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

const (
	// crimesResourceID is the Chicago Data Portal's "Crimes - 2001 to
	// Present" dataset -- see docs/DATA_SOURCES.md "Crimes".
	crimesResourceID = "ijzp-q8t2"
	// crimesPageSize balances request count against per-request payload
	// size for a ~225K-row trailing-12-month window (~45 requests).
	crimesPageSize = 5000
)

type crimeRow struct {
	ID          string `json:"id"`
	Date        string `json:"date"`
	PrimaryType string `json:"primary_type"`
	IUCR        string `json:"iucr"`
	Latitude    string `json:"latitude"`
	Longitude   string `json:"longitude"`
}

// ingestCrimes replaces cpd_crimes with the trailing 12 months of Chicago
// Data Portal crime records, classified via classifyCrime (see
// crime_categories.go).
//
// Pages through the source using id-based keyset pagination (ORDER BY id,
// WHERE id > <last seen>) rather than $offset -- Socrata's own docs say
// offset paging degrades as it goes deeper, and id is confirmed number-typed
// in the dataset's own column metadata, so numeric ordering is safe. See
// docs/DATA_SOURCES.md "Crimes" "Pagination".
//
// Each page is classified and inserted immediately as one pgx.Batch, rather
// than accumulating all ~225K rows in memory first -- the whole run still
// commits as a single transaction, so a failure partway through leaves
// cpd_crimes untouched rather than half-replaced.
func ingestCrimes(ctx context.Context, soda *clients.SODAClient, pool *pgxpool.Pool) error {
	cutoff := time.Now().AddDate(-1, 0, 0).Format("2006-01-02T15:04:05")

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, "DELETE FROM cpd_crimes"); err != nil {
		return fmt.Errorf("clear existing crimes: %w", err)
	}

	var (
		lastID                                                     int64
		fetched, inserted, skippedNoCoords, excluded, unrecognized int
		unrecognizedCounts                                         = map[string]int{} // "primary_type|iucr" -> count
	)

	for {
		var rows []crimeRow
		params := url.Values{
			"$where": {fmt.Sprintf("date > '%s' AND id > %d", cutoff, lastID)},
			"$order": {"id"},
			"$limit": {strconv.Itoa(crimesPageSize)},
		}
		if err := soda.GetJSON(ctx, crimesResourceID, params, &rows); err != nil {
			return fmt.Errorf("fetch crimes page (after id %d): %w", lastID, err)
		}
		if len(rows) == 0 {
			break
		}
		fetched += len(rows)

		batch := &pgx.Batch{}
		queued := 0
		for _, row := range rows {
			id, err := strconv.ParseInt(row.ID, 10, 64)
			if err != nil {
				return fmt.Errorf("parse crime id %q: %w", row.ID, err)
			}
			if id > lastID {
				lastID = id
			}

			if row.Latitude == "" || row.Longitude == "" {
				skippedNoCoords++
				continue
			}

			category, ok := classifyCrime(row.PrimaryType, row.IUCR)
			if !ok {
				unrecognized++
				unrecognizedCounts[row.PrimaryType+"|"+row.IUCR]++
				continue
			}
			if category == categoryExcluded {
				excluded++
				continue
			}

			lon, err := strconv.ParseFloat(row.Longitude, 64)
			if err != nil {
				return fmt.Errorf("parse longitude for crime %s: %w", row.ID, err)
			}
			lat, err := strconv.ParseFloat(row.Latitude, 64)
			if err != nil {
				return fmt.Errorf("parse latitude for crime %s: %w", row.ID, err)
			}
			// date is Socrata's floating_timestamp (no timezone attached).
			// Parsed and stored as UTC -- exact time-of-day precision isn't
			// meaningful for S_safety's trailing-12-month window bucketing.
			crimeDate, err := time.Parse("2006-01-02T15:04:05.000", row.Date)
			if err != nil {
				return fmt.Errorf("parse date for crime %s: %w", row.ID, err)
			}

			batch.Queue(
				`INSERT INTO cpd_crimes (source_id, crime_date, primary_type, iucr, category, geom)
				 VALUES ($1, $2, $3, $4, $5, ST_SetSRID(ST_MakePoint($6, $7), 4326)::geography)`,
				row.ID, crimeDate, row.PrimaryType, row.IUCR, category, lon, lat,
			)
			queued++
		}

		if queued > 0 {
			results := tx.SendBatch(ctx, batch)
			for i := 0; i < queued; i++ {
				if _, err := results.Exec(); err != nil {
					results.Close() //nolint:errcheck // already returning the real error
					return fmt.Errorf("insert crime batch (item %d of %d): %w", i+1, queued, err)
				}
			}
			if err := results.Close(); err != nil {
				return fmt.Errorf("close crime batch results: %w", err)
			}
			inserted += queued
		}

		if len(rows) < crimesPageSize {
			break
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	slog.Info("crimes ingestion summary",
		slog.Int("fetched", fetched),
		slog.Int("inserted", inserted),
		slog.Int("skipped_no_coords", skippedNoCoords),
		slog.Int("excluded", excluded),
		slog.Int("unrecognized", unrecognized),
	)
	// A WARN here is deliberate, not an error: an unrecognized combination
	// is skipped safely, but should be visible so the taxonomy in
	// docs/DATA_SOURCES.md gets extended rather than silently drifting.
	// See IMPLEMENTATION_PHASES.md Phase 7 for the deferred Loki alert on
	// this line.
	for key, count := range unrecognizedCounts {
		slog.Warn("unrecognized crime classification", slog.String("primary_type|iucr", key), slog.Int("count", count))
	}

	return nil
}
