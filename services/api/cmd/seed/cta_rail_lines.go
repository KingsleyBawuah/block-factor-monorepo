package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

// ctaRailLinesResourceID is the Chicago Data Portal's "CTA - 'L' (Rail)
// Lines" dataset -- see docs/DATA_SOURCES.md "CTA 'L' Lines". This is the
// only real per-line rail track geometry available: cta_stops has only
// station points, and noise_sources' CTA track geometry is line-agnostic.
// This dataset has no published unique-ID field of its own, so source_id
// uses Socrata's internal :id row identifier (requested via $select).
const ctaRailLinesResourceID = "xbyr-jnvx"

type ctaRailLineRow struct {
	SourceID string          `json:"id"` // aliased from Socrata's :id via $select
	TheGeom  json.RawMessage `json:"the_geom"`
	Lines    string          `json:"lines"`
	Type     string          `json:"type"`
}

// parseRailLines splits a raw Chicago Data Portal `lines` field (e.g.
// "Brown, Purple (Express), Red") into the canonical line names matching
// cta_stops.lines' vocabulary (Red, Blue, Brown, Green, Orange, Pink,
// Purple, Yellow), plus whether any line on this segment runs express
// service. See docs/DATA_SOURCES.md "CTA 'L' Lines" for the real, verified
// set of raw values this handles -- notably the inconsistent
// "(Express)"/"(Exp)" abbreviation, and Blue Line's two branch qualifiers
// ("(Forest Park)"/"(O'Hare)"), which are not express modifiers and are
// discarded rather than stored.
func parseRailLines(raw string) (lines []string, express bool) {
	for _, token := range strings.Split(raw, ",") {
		token = strings.TrimSpace(token)

		if s, ok := strings.CutSuffix(token, "(Express)"); ok {
			token, express = strings.TrimSpace(s), true
		} else if s, ok := strings.CutSuffix(token, "(Exp)"); ok {
			token, express = strings.TrimSpace(s), true
		} else if s, ok := strings.CutSuffix(token, "(Forest Park)"); ok {
			token = strings.TrimSpace(s)
		} else if s, ok := strings.CutSuffix(token, "(O'Hare)"); ok {
			token = strings.TrimSpace(s)
		}

		token = strings.TrimSpace(strings.TrimSuffix(token, " Line"))
		lines = append(lines, token)
	}
	return lines, express
}

// railGrade maps the source's `type` field to cta_rail_lines.grade.
func railGrade(sourceType string) (string, error) {
	switch sourceType {
	case "Subway":
		return "subway", nil
	case "Elevated or At Grade":
		return "elevated_or_at_grade", nil
	default:
		return "", fmt.Errorf("unrecognized rail line type %q", sourceType)
	}
}

// ingestCTARailLines replaces cta_rail_lines with the current Chicago Data
// Portal per-line 'L' track geometry.
func ingestCTARailLines(ctx context.Context, soda *clients.SODAClient, pool *pgxpool.Pool) error {
	var rows []ctaRailLineRow
	params := url.Values{
		"$select": {":id AS id, the_geom, lines, type"},
		"$limit":  {"1000"}, // source has 153 rows
	}
	if err := soda.GetJSON(ctx, ctaRailLinesResourceID, params, &rows); err != nil {
		return fmt.Errorf("fetch CTA rail lines: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, "DELETE FROM cta_rail_lines"); err != nil {
		return fmt.Errorf("clear existing CTA rail lines: %w", err)
	}

	const insertSQL = `
		INSERT INTO cta_rail_lines (source_id, lines, express, grade, geom)
		VALUES ($1, $2, $3, $4, ST_Multi(ST_GeomFromGeoJSON($5))::geography)
	`
	for _, row := range rows {
		lines, express := parseRailLines(row.Lines)
		grade, err := railGrade(row.Type)
		if err != nil {
			return fmt.Errorf("rail line segment %s: %w", row.SourceID, err)
		}
		if _, err := tx.Exec(ctx, insertSQL, row.SourceID, lines, express, grade, row.TheGeom); err != nil {
			return fmt.Errorf("insert CTA rail line segment %s: %w", row.SourceID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
