package main

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

// ctaStopsResourceID is the Chicago Data Portal's "CTA - System Information
// - List of 'L' Stops" dataset. It has one row per directional platform
// (e.g. "Howard (95th-Bound)" and "Howard (Linden, Skokie-bound)" are two
// separate rows for the same physical station), not per station -- see
// docs/DATA_SOURCES.md "CTA 'L' Stops". This parser groups rows by map_id
// (shared across a station's directional rows) into one cta_stops row per
// station, unioning whichever line flags are true across the group.
const ctaStopsResourceID = "8pix-ypme"

type ctaStopRow struct {
	MapID       string `json:"map_id"`
	StationName string `json:"station_name"`
	ADA         bool   `json:"ada"`
	Red         bool   `json:"red"`
	Blue        bool   `json:"blue"`
	Green       bool   `json:"g"`
	Brown       bool   `json:"brn"`
	Purple      bool   `json:"p"`
	Yellow      bool   `json:"y"`
	Pink        bool   `json:"pnk"`
	Orange      bool   `json:"o"`
	Location    struct {
		Latitude  string `json:"latitude"`
		Longitude string `json:"longitude"`
	} `json:"location"`
}

type ctaStation struct {
	stationName                                           string
	latitude, longitude                                   string
	ada                                                   bool
	red, blue, green, brown, purple, yellow, pink, orange bool
}

func (s *ctaStation) lines() []string {
	var lines []string
	if s.red {
		lines = append(lines, "Red")
	}
	if s.blue {
		lines = append(lines, "Blue")
	}
	if s.green {
		lines = append(lines, "Green")
	}
	if s.brown {
		lines = append(lines, "Brown")
	}
	if s.purple {
		lines = append(lines, "Purple")
	}
	if s.yellow {
		lines = append(lines, "Yellow")
	}
	if s.pink {
		lines = append(lines, "Pink")
	}
	if s.orange {
		lines = append(lines, "Orange")
	}
	return lines
}

// ingestCTAStops replaces cta_stops with the current Chicago Data Portal
// stations, grouped from the source's per-direction rows.
func ingestCTAStops(ctx context.Context, soda *clients.SODAClient, pool *pgxpool.Pool) error {
	var rows []ctaStopRow
	params := url.Values{"$limit": {"5000"}} // source has 302 directional rows; well above that
	if err := soda.GetJSON(ctx, ctaStopsResourceID, params, &rows); err != nil {
		return fmt.Errorf("fetch CTA stops: %w", err)
	}

	stations := make(map[string]*ctaStation)
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		st, ok := stations[row.MapID]
		if !ok {
			st = &ctaStation{
				stationName: row.StationName,
				latitude:    row.Location.Latitude,
				longitude:   row.Location.Longitude,
			}
			stations[row.MapID] = st
			order = append(order, row.MapID)
		}
		st.ada = st.ada || row.ADA
		st.red = st.red || row.Red
		st.blue = st.blue || row.Blue
		st.green = st.green || row.Green
		st.brown = st.brown || row.Brown
		st.purple = st.purple || row.Purple
		st.yellow = st.yellow || row.Yellow
		st.pink = st.pink || row.Pink
		st.orange = st.orange || row.Orange
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, "DELETE FROM cta_stops"); err != nil {
		return fmt.Errorf("clear existing CTA stops: %w", err)
	}

	const insertSQL = `
		INSERT INTO cta_stops (source_id, stop_name, lines, ada, geom)
		VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326)::geography)
	`
	for _, mapID := range order {
		st := stations[mapID]

		lon, err := strconv.ParseFloat(st.longitude, 64)
		if err != nil {
			return fmt.Errorf("parse longitude for station %s (%s): %w", mapID, st.stationName, err)
		}
		lat, err := strconv.ParseFloat(st.latitude, 64)
		if err != nil {
			return fmt.Errorf("parse latitude for station %s (%s): %w", mapID, st.stationName, err)
		}

		if _, err := tx.Exec(ctx, insertSQL, mapID, st.stationName, st.lines(), st.ada, lon, lat); err != nil {
			return fmt.Errorf("insert CTA station %s (%s): %w", mapID, st.stationName, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
