package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

const (
	// sewer311ResourceID is the Chicago Data Portal's unified "311 Service
	// Requests" dataset -- see docs/DATA_SOURCES.md "311 Service Requests".
	sewer311ResourceID = "v6vf-nfxy"
	// sewer311SRType is the verified service request type for basement
	// water/sewer backups powering S_flood (PRD 5.7).
	sewer311SRType = "Water in Basement Complaint"
	// sewer311PageSize balances request count against per-request payload
	// size for a ~25K-row trailing-24-month window (~5-6 requests).
	sewer311PageSize = 5000
	// sewer311InsertBatchSize caps statement count per pgx.Batch.
	sewer311InsertBatchSize = 5000
)

type sewer311Row struct {
	SRNumber    string `json:"sr_number"`
	SRType      string `json:"sr_type"`
	CreatedDate string `json:"created_date"`
	Duplicate   bool   `json:"duplicate"`
	Latitude    string `json:"latitude"`
	Longitude   string `json:"longitude"`
}

type sewer311Insert struct {
	sourceID string
	date     time.Time
	lon, lat float64
}

// parseSewer311Row validates a single 311 row and extracts the insert fields.
// Returns ok=false if the row has missing coordinates.
func parseSewer311Row(row sewer311Row) (sewer311Insert, bool, error) {
	if row.SRNumber == "" {
		return sewer311Insert{}, false, fmt.Errorf("empty sr_number")
	}
	if row.Latitude == "" || row.Longitude == "" {
		return sewer311Insert{}, false, nil
	}

	lon, err := strconv.ParseFloat(row.Longitude, 64)
	if err != nil {
		return sewer311Insert{}, false, fmt.Errorf("parse longitude for 311 incident %s: %w", row.SRNumber, err)
	}
	lat, err := strconv.ParseFloat(row.Latitude, 64)
	if err != nil {
		return sewer311Insert{}, false, fmt.Errorf("parse latitude for 311 incident %s: %w", row.SRNumber, err)
	}

	// created_date is Socrata's floating_timestamp (e.g. "2024-08-23T09:05:36.000").
	// Strip fractional seconds to handle variable digit precision safely.
	datePart, _, _ := strings.Cut(row.CreatedDate, ".")
	incidentDate, err := time.Parse("2006-01-02T15:04:05", datePart)
	if err != nil {
		return sewer311Insert{}, false, fmt.Errorf("parse created_date for 311 incident %s: %w", row.SRNumber, err)
	}

	return sewer311Insert{
		sourceID: row.SRNumber,
		date:     incidentDate,
		lon:      lon,
		lat:      lat,
	}, true, nil
}

// ingestSewer311 replaces sewer_311_incidents with the trailing 24 months of
// Chicago Data Portal water in basement 311 complaints, filtered to
// duplicate = false to avoid penalizing multiple calls for the same event.
//
// Uses sr_number keyset pagination (ORDER BY sr_number, WHERE sr_number > <last seen>).
// Fetches all rows into memory first, then opens a short DB transaction (TRUNCATE + batched insert).
func ingestSewer311(ctx context.Context, soda *clients.SODAClient, pool *pgxpool.Pool) error {
	cutoff := time.Now().AddDate(-2, 0, 0).Format("2006-01-02T15:04:05")

	var (
		lastSRNumber    string
		fetched         int
		skippedNoCoords int
		toInsert        []sewer311Insert
		page            int
	)

	for {
		var rows []sewer311Row
		whereClause := fmt.Sprintf("sr_type = '%s' AND duplicate = false AND created_date >= '%s'", sewer311SRType, cutoff)
		if lastSRNumber != "" {
			whereClause = fmt.Sprintf("%s AND sr_number > '%s'", whereClause, lastSRNumber)
		}

		params := url.Values{
			"$where": {whereClause},
			"$order": {"sr_number"},
			"$limit": {strconv.Itoa(sewer311PageSize)},
		}

		if err := soda.GetJSON(ctx, sewer311ResourceID, params, &rows); err != nil {
			return fmt.Errorf("fetch sewer 311 page (after sr_number %q): %w", lastSRNumber, err)
		}
		if len(rows) == 0 {
			break
		}

		page++
		fetched += len(rows)

		for _, row := range rows {
			lastSRNumber = row.SRNumber

			parsed, ok, err := parseSewer311Row(row)
			if err != nil {
				return err
			}
			if !ok {
				skippedNoCoords++
				continue
			}

			toInsert = append(toInsert, parsed)
		}

		slog.Info("sewer 311 fetch progress", slog.Int("pages", page), slog.Int("fetched", fetched))

		if len(rows) < sewer311PageSize {
			break
		}
	}

	if err := insertSewer311(ctx, pool, toInsert); err != nil {
		return err
	}

	slog.Info("sewer 311 ingestion summary",
		slog.Int("fetched", fetched),
		slog.Int("inserted", len(toInsert)),
		slog.Int("skipped_no_coords", skippedNoCoords),
	)

	return nil
}

// insertSewer311 replaces sewer_311_incidents in a single fast transaction.
func insertSewer311(ctx context.Context, pool *pgxpool.Pool, rows []sewer311Insert) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, "TRUNCATE sewer_311_incidents"); err != nil {
		return fmt.Errorf("truncate existing sewer 311 incidents: %w", err)
	}

	const insertSQL = `
		INSERT INTO sewer_311_incidents (source_id, incident_date, geom)
		VALUES ($1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography)
	`
	for start := 0; start < len(rows); start += sewer311InsertBatchSize {
		end := min(start+sewer311InsertBatchSize, len(rows))
		chunk := rows[start:end]

		batch := &pgx.Batch{}
		for _, r := range chunk {
			batch.Queue(insertSQL, r.sourceID, r.date, r.lon, r.lat)
		}

		results := tx.SendBatch(ctx, batch)
		for i := range chunk {
			if _, err := results.Exec(); err != nil {
				results.Close() //nolint:errcheck // already returning the real error
				return fmt.Errorf("insert sewer 311 batch (item %d of %d): %w", i+1, len(chunk), err)
			}
		}
		if err := results.Close(); err != nil {
			return fmt.Errorf("close sewer 311 batch results: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
