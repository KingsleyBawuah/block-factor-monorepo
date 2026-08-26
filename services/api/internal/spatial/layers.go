package spatial

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LayerReader serves the Phase 4 static map layers as ready-to-render
// GeoJSON FeatureCollections. Each method returns the raw JSON bytes
// Postgres itself assembled (via jsonb_build_object/jsonb_agg), rather than
// unmarshaling into Go structs first -- the handler writes the bytes
// straight through as the HTTP response body. See
// docs/IMPLEMENTATION_PHASES.md Phase 4 deliverable 3.
type LayerReader interface {
	CTARailLinesGeoJSON(ctx context.Context) ([]byte, error)
	CTAStationsGeoJSON(ctx context.Context) ([]byte, error)
	BikeRoutesGeoJSON(ctx context.Context) ([]byte, error)
	TrailAccessGeoJSON(ctx context.Context) ([]byte, error)
	SupermarketsGeoJSON(ctx context.Context) ([]byte, error)
}

// DBLayerReader implements LayerReader against the PostGIS connection pool.
type DBLayerReader struct {
	pool *pgxpool.Pool
}

// NewDBLayerReader creates a LayerReader backed by the PostGIS connection pool.
func NewDBLayerReader(pool *pgxpool.Pool) *DBLayerReader {
	return &DBLayerReader{pool: pool}
}

// featureCollectionQuery runs a query expected to return exactly one row/one
// jsonb column -- the fully-assembled FeatureCollection -- and returns its
// raw bytes. COALESCE around jsonb_agg in each query below guarantees
// "features" is always a JSON array (never null) even when the underlying
// table/filter matches zero rows, matching docs/openapi.yaml's non-nullable
// array schema.
func (r *DBLayerReader) featureCollectionQuery(ctx context.Context, query string) ([]byte, error) {
	if r.pool == nil {
		return nil, errors.New("spatial: database pool is nil")
	}

	var raw []byte
	if err := r.pool.QueryRow(ctx, query).Scan(&raw); err != nil {
		return nil, fmt.Errorf("spatial: query layer: %w", err)
	}
	return raw, nil
}

const ctaRailLinesGeoJSONQuery = `
	SELECT jsonb_build_object(
		'type', 'FeatureCollection',
		'features', COALESCE(jsonb_agg(
			jsonb_build_object(
				'type', 'Feature',
				'properties', jsonb_build_object('lines', lines, 'express', express, 'grade', grade),
				'geometry', ST_AsGeoJSON(geom)::jsonb
			) ORDER BY id
		), '[]'::jsonb)
	)
	FROM cta_rail_lines
`

// CTARailLinesGeoJSON returns every CTA rail track segment as a GeoJSON
// FeatureCollection of MultiLineString features.
func (r *DBLayerReader) CTARailLinesGeoJSON(ctx context.Context) ([]byte, error) {
	return r.featureCollectionQuery(ctx, ctaRailLinesGeoJSONQuery)
}

const ctaStationsGeoJSONQuery = `
	SELECT jsonb_build_object(
		'type', 'FeatureCollection',
		'features', COALESCE(jsonb_agg(
			jsonb_build_object(
				'type', 'Feature',
				'properties', jsonb_build_object('name', stop_name, 'lines', lines, 'ada', ada),
				'geometry', ST_AsGeoJSON(geom)::jsonb
			) ORDER BY id
		), '[]'::jsonb)
	)
	FROM cta_stops
`

// CTAStationsGeoJSON returns every CTA 'L' station as a GeoJSON
// FeatureCollection of Point features.
func (r *DBLayerReader) CTAStationsGeoJSON(ctx context.Context) ([]byte, error) {
	return r.featureCollectionQuery(ctx, ctaStationsGeoJSONQuery)
}

const bikeRoutesGeoJSONQuery = `
	SELECT jsonb_build_object(
		'type', 'FeatureCollection',
		'features', COALESCE(jsonb_agg(
			jsonb_build_object(
				'type', 'Feature',
				'properties', jsonb_build_object('routeType', route_type, 'streetName', street_name),
				'geometry', ST_AsGeoJSON(geom)::jsonb
			) ORDER BY id
		), '[]'::jsonb)
	)
	FROM bike_routes
`

// BikeRoutesGeoJSON returns every bike route segment as a GeoJSON
// FeatureCollection of MultiLineString features.
func (r *DBLayerReader) BikeRoutesGeoJSON(ctx context.Context) ([]byte, error) {
	return r.featureCollectionQuery(ctx, bikeRoutesGeoJSONQuery)
}

// trailAccessGeoJSONQuery reads from parks WHERE park_class = 'LINEAR PARK'
// -- the real source of off-street trail access (Lakefront Trail, The
// 606/Bloomingdale Trail) per the PRD §5.5 schema decision. bike_routes has
// no trail category at all -- see docs/DATA_SOURCES.md "Parks".
const trailAccessGeoJSONQuery = `
	SELECT jsonb_build_object(
		'type', 'FeatureCollection',
		'features', COALESCE(jsonb_agg(
			jsonb_build_object(
				'type', 'Feature',
				'properties', jsonb_build_object('name', park_name),
				'geometry', ST_AsGeoJSON(geom)::jsonb
			) ORDER BY id
		), '[]'::jsonb)
	)
	FROM parks
	WHERE park_class = 'LINEAR PARK'
`

// TrailAccessGeoJSON returns off-street trail (linear park) boundaries as a
// GeoJSON FeatureCollection of MultiPolygon features.
func (r *DBLayerReader) TrailAccessGeoJSON(ctx context.Context) ([]byte, error) {
	return r.featureCollectionQuery(ctx, trailAccessGeoJSONQuery)
}

const supermarketsGeoJSONQuery = `
	SELECT jsonb_build_object(
		'type', 'FeatureCollection',
		'features', COALESCE(jsonb_agg(
			jsonb_build_object(
				'type', 'Feature',
				'properties', jsonb_build_object('name', poi_name),
				'geometry', ST_AsGeoJSON(geom)::jsonb
			) ORDER BY id
		), '[]'::jsonb)
	)
	FROM osm_pois
	WHERE poi_type = 'supermarket'
`

// SupermarketsGeoJSON returns every supermarket as a GeoJSON
// FeatureCollection of Point features.
func (r *DBLayerReader) SupermarketsGeoJSON(ctx context.Context) ([]byte, error) {
	return r.featureCollectionQuery(ctx, supermarketsGeoJSONQuery)
}
