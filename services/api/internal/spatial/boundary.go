package spatial

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// QueryRowProvider is satisfied by *pgxpool.Pool, *pgx.Conn, and pgx.Tx.
type QueryRowProvider interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// BoundaryChecker checks whether coordinates fall inside the active service coverage area.
type BoundaryChecker interface {
	IsWithinCityBoundary(ctx context.Context, lat, lon float64) (bool, error)
}

// DBBoundaryChecker implements BoundaryChecker against the PostGIS city_boundary table.
type DBBoundaryChecker struct {
	pool *pgxpool.Pool
}

// NewDBBoundaryChecker creates a BoundaryChecker backed by the PostGIS connection pool.
func NewDBBoundaryChecker(pool *pgxpool.Pool) *DBBoundaryChecker {
	return &DBBoundaryChecker{pool: pool}
}

// IsWithinCityBoundary checks if the given coordinates fall inside the City of Chicago boundary polygon.
func (c *DBBoundaryChecker) IsWithinCityBoundary(ctx context.Context, lat, lon float64) (bool, error) {
	if c.pool == nil {
		return false, errors.New("spatial: database pool is nil")
	}
	return IsWithinCityBoundary(ctx, c.pool, lat, lon)
}

// IsWithinCityBoundary checks whether the point (lat, lon) is covered by the city_boundary polygon.
// In PostGIS, ST_MakePoint takes (X, Y) which corresponds to (lon, lat) in SRID 4326.
// ST_Covers tests whether the multi-polygon contains the point or lands on its boundary.
func IsWithinCityBoundary(ctx context.Context, db QueryRowProvider, lat, lon float64) (bool, error) {
	if db == nil {
		return false, errors.New("spatial: nil database connection")
	}

	query := `
		SELECT ST_Covers(geom::geometry, ST_SetSRID(ST_MakePoint($1, $2), 4326))
		FROM city_boundary
		LIMIT 1;
	`

	var isCovered bool
	err := db.QueryRow(ctx, query, lon, lat).Scan(&isCovered)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, errors.New("spatial: city_boundary table is empty; please run `make seed --dataset=boundary`")
		}
		return false, fmt.Errorf("spatial: query city boundary: %w", err)
	}

	return isCovered, nil
}
