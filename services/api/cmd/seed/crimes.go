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
	// crimesResourceID is the Chicago Data Portal's "Crimes - 2001 to
	// Present" dataset -- see docs/DATA_SOURCES.md "Crimes".
	crimesResourceID = "ijzp-q8t2"
	// crimesPageSize balances request count against per-request payload
	// size for a ~225K-row trailing-12-month window (~45 requests).
	crimesPageSize = 5000
	// crimesInsertBatchSize caps how many statements go into a single
	// pgx.Batch, so one round trip stays a bounded size even though the
	// whole insert phase covers ~225K rows.
	crimesInsertBatchSize = 5000
)

type crimeRow struct {
	ID          string `json:"id"`
	Date        string `json:"date"`
	PrimaryType string `json:"primary_type"`
	IUCR        string `json:"iucr"`
	Latitude    string `json:"latitude"`
	Longitude   string `json:"longitude"`
}

// crimeInsert is a crimeRow that has already been validated and classified,
// ready to insert.
type crimeInsert struct {
	sourceID    string
	date        time.Time
	primaryType string
	iucr        string
	category    string
	lon, lat    float64
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
// The ~45-page fetch happens entirely before any database transaction
// opens: earlier revisions ran DELETE+insert inside one transaction that
// stayed open for the whole ~80s fetch, which is bad practice regardless of
// the exact MVCC visibility details -- it needlessly holds locks and a
// snapshot open for the duration of 45 sequential external HTTP calls, and
// ties transaction length to that API's reliability rather than to the
// (fast) DB work. Classified rows are accumulated in memory (~225K rows,
// tens of MB) instead, and the transaction that follows only does DB work:
// TRUNCATE, then batched inserts, then commit -- seconds, not a minute-plus.
func ingestCrimes(ctx context.Context, soda *clients.SODAClient, pool *pgxpool.Pool) error {
	cutoff := time.Now().AddDate(-1, 0, 0).Format("2006-01-02T15:04:05")

	var (
		lastID                                           int64
		fetched, skippedNoCoords, excluded, unrecognized int
		unrecognizedCounts                               = map[string]int{} // "primary_type|iucr" -> count
		toInsert                                         []crimeInsert
		page                                             int
	)

	for {
		var rows []crimeRow
		params := url.Values{
			// cutoff and lastID are both self-produced (formatted
			// time.Now() and a previously-fetched numeric id), never
			// external input -- Socrata has no parameterized-query
			// mechanism, so this $where is built via Sprintf rather than
			// bind parameters. Keep it that way: if this clause is ever
			// extended with a value that originates outside this
			// function (e.g. a future --primary-type flag), it must be
			// validated/allowlisted before landing here, not interpolated
			// directly.
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
		page++

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
			// date is Socrata's floating_timestamp (no timezone attached),
			// e.g. "2026-08-15T00:00:00.000". The fractional-second
			// component's digit count isn't guaranteed by Socrata's docs
			// (and a fixed-width layout like "...05.000" requires an exact
			// digit count to parse, confirmed by testing against Go's
			// time.Parse directly -- it does NOT make fractional digits
			// optional the way a literal reading of the layout might
			// suggest). Stripping everything from the first "." onward
			// sidesteps the whole question: exact time-of-day precision
			// isn't meaningful for S_safety's trailing-12-month window
			// bucketing anyway.
			datePart, _, _ := strings.Cut(row.Date, ".")
			crimeDate, err := time.Parse("2006-01-02T15:04:05", datePart)
			if err != nil {
				return fmt.Errorf("parse date for crime %s: %w", row.ID, err)
			}

			toInsert = append(toInsert, crimeInsert{
				sourceID:    row.ID,
				date:        crimeDate,
				primaryType: row.PrimaryType,
				iucr:        row.IUCR,
				category:    category,
				lon:         lon,
				lat:         lat,
			})
		}

		if page%10 == 0 {
			slog.Info("crimes fetch progress", slog.Int("pages", page), slog.Int("fetched", fetched))
		}

		if len(rows) < crimesPageSize {
			break
		}
	}

	if err := insertCrimes(ctx, pool, toInsert); err != nil {
		return err
	}

	slog.Info("crimes ingestion summary",
		slog.Int("fetched", fetched),
		slog.Int("inserted", len(toInsert)),
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

// insertCrimes replaces cpd_crimes with rows in a single transaction whose
// only work is DB-local: TRUNCATE, then batched inserts, then commit. Using
// TRUNCATE instead of DELETE avoids WAL entries proportional to the
// (typically 200K+) row count being replaced; its stronger ACCESS EXCLUSIVE
// lock (which briefly blocks readers too, unlike DELETE) is an acceptable
// trade here specifically because this transaction is now short -- seconds,
// not the ~80s the fetch phase takes.
func insertCrimes(ctx context.Context, pool *pgxpool.Pool, rows []crimeInsert) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, "TRUNCATE cpd_crimes"); err != nil {
		return fmt.Errorf("truncate existing crimes: %w", err)
	}

	const insertSQL = `
		INSERT INTO cpd_crimes (source_id, crime_date, primary_type, iucr, category, geom)
		VALUES ($1, $2, $3, $4, $5, ST_SetSRID(ST_MakePoint($6, $7), 4326)::geography)
	`
	for start := 0; start < len(rows); start += crimesInsertBatchSize {
		end := min(start+crimesInsertBatchSize, len(rows))
		chunk := rows[start:end]

		batch := &pgx.Batch{}
		for _, r := range chunk {
			batch.Queue(insertSQL, r.sourceID, r.date, r.primaryType, r.iucr, r.category, r.lon, r.lat)
		}

		results := tx.SendBatch(ctx, batch)
		for i := range chunk {
			if _, err := results.Exec(); err != nil {
				results.Close() //nolint:errcheck // already returning the real error
				return fmt.Errorf("insert crime batch (item %d of %d): %w", i+1, len(chunk), err)
			}
		}
		if err := results.Close(); err != nil {
			return fmt.Errorf("close crime batch results: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
