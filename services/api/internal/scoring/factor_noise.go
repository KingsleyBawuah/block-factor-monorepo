package scoring

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/spatial"
)

// Noise calculation threshold and penalty constants (PRD §5.6).
const (
	// NoiseMaxQueryRadiusMeters is 500 ft (152.4 m), the maximum radius for noise penalties.
	NoiseMaxQueryRadiusMeters = 152.4

	// NoiseTrackHighImpactThresholdMeters is 150 ft (45.72 m) for -35 pt CTA track penalty.
	NoiseTrackHighImpactThresholdMeters = 45.72
	// NoiseTrackHighImpactPenalty is the point deduction for CTA tracks within 150 ft.
	NoiseTrackHighImpactPenalty = 35.0

	// NoiseTrackMediumImpactThresholdMeters is 300 ft (91.44 m) for -20 pt CTA track penalty.
	NoiseTrackMediumImpactThresholdMeters = 91.44
	// NoiseTrackMediumImpactPenalty is the point deduction for CTA tracks between 150-300 ft.
	NoiseTrackMediumImpactPenalty = 20.0

	// NoiseExpresswayThresholdMeters is 500 ft (152.4 m) for -30 pt expressway penalty.
	NoiseExpresswayThresholdMeters = 152.4
	// NoiseExpresswayPenalty is the point deduction for expressways within 500 ft.
	NoiseExpresswayPenalty = 30.0

	// NoiseArterialThresholdMeters is 100 ft (30.48 m) for -15 pt 4-lane arterial penalty.
	NoiseArterialThresholdMeters = 30.48
	// NoiseArterialPenalty is the point deduction for major 4-lane arterials within 100 ft.
	NoiseArterialPenalty = 15.0
)

// NoiseProximity represents a nearby urban noise generator and its geodesic distance.
type NoiseProximity struct {
	SourceType     string  // 'cta_track', 'expressway', 'arterial'
	Grade          *string // 'elevated', 'at_grade', 'underground' (CTA tracks only)
	DistanceMeters float64
}

// NoiseQuerier queries nearby noise generators in PostGIS.
type NoiseQuerier interface {
	GetNearbyNoiseSources(ctx context.Context, lat, lon float64, maxDistanceMeters float64) ([]NoiseProximity, error)
}

// DBNoiseQuerier executes noise queries against PostGIS.
type DBNoiseQuerier struct {
	db spatial.QueryProvider
}

// NewDBNoiseQuerier creates a new DBNoiseQuerier.
func NewDBNoiseQuerier(db spatial.QueryProvider) *DBNoiseQuerier {
	return &DBNoiseQuerier{db: db}
}

// GetNearbyNoiseSources returns noise sources within maxDistanceMeters of the coordinate.
func (q *DBNoiseQuerier) GetNearbyNoiseSources(ctx context.Context, lat, lon float64, maxDistanceMeters float64) ([]NoiseProximity, error) {
	if q.db == nil {
		return nil, errors.New("scoring: nil database connection")
	}

	query := `
		SELECT
			source_type,
			grade,
			ST_Distance(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) AS distance_meters
		FROM noise_sources
		WHERE ST_DWithin(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
		ORDER BY distance_meters ASC;
	`

	rows, err := q.db.Query(ctx, query, lon, lat, maxDistanceMeters)
	if err != nil {
		return nil, fmt.Errorf("scoring: query nearby noise sources: %w", err)
	}
	defer rows.Close()

	var sources []NoiseProximity
	for rows.Next() {
		var s NoiseProximity
		if err := rows.Scan(&s.SourceType, &s.Grade, &s.DistanceMeters); err != nil {
			return nil, fmt.Errorf("scoring: scan noise source row: %w", err)
		}
		sources = append(sources, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scoring: iterate noise rows: %w", err)
	}

	return sources, nil
}

// NoiseCalculator calculates the S_noise acoustic livability factor.
type NoiseCalculator struct {
	querier NoiseQuerier
}

// NewNoiseCalculator creates a new NoiseCalculator.
func NewNoiseCalculator(querier NoiseQuerier) *NoiseCalculator {
	return &NoiseCalculator{querier: querier}
}

// Name returns FactorNoise.
func (c *NoiseCalculator) Name() FactorName {
	return FactorNoise
}

// Calculate computes S_noise for the given origin coordinates.
func (c *NoiseCalculator) Calculate(ctx context.Context, params ScoreParams) (FactorResult, error) {
	if c.querier == nil {
		return FactorResult{Name: FactorNoise, Degraded: true}, errors.New("scoring: noise querier not configured")
	}

	sources, err := c.querier.GetNearbyNoiseSources(ctx, params.Origin.Lat, params.Origin.Lon, NoiseMaxQueryRadiusMeters)
	if err != nil {
		return FactorResult{Name: FactorNoise, Degraded: true}, err
	}

	score, minDistance := CalculateNoiseScore(sources)
	return FactorResult{
		Name:           FactorNoise,
		Score:          &score,
		DistanceMeters: minDistance,
		Degraded:       false,
	}, nil
}

// CalculateNoiseScore computes acoustic livability from nearby noise generators (PRD §5.6):
//   - Base score: 100 pts (quiet residential street baseline).
//   - Elevated/At-Grade CTA Track:
//   - < 150 ft (45.72 m): -35 pts
//   - 150–300 ft (45.72–91.44 m): -20 pts
//   - Underground (subway) segments are exempt (0 pts penalty).
//   - Expressway:
//   - < 500 ft (152.4 m): -30 pts
//   - 4-lane Arterial:
//   - < 100 ft (30.48 m): -15 pts
//   - Penalties are cumulative and floored at 0: max(0, 100 - sum(Penalties)).
func CalculateNoiseScore(sources []NoiseProximity) (score float32, minContributingDistance *float32) {
	var nearestTrackDist *float64
	var nearestExpresswayDist *float64
	var nearestArterialDist *float64

	for _, s := range sources {
		switch s.SourceType {
		case "cta_track":
			// Only elevated and at-grade CTA tracks generate noise penalties (PRD §5.6)
			if s.Grade != nil {
				grade := strings.ToLower(*s.Grade)
				if grade == "elevated" || grade == "at_grade" {
					if nearestTrackDist == nil || s.DistanceMeters < *nearestTrackDist {
						dist := s.DistanceMeters
						nearestTrackDist = &dist
					}
				}
			}
		case "expressway":
			if nearestExpresswayDist == nil || s.DistanceMeters < *nearestExpresswayDist {
				dist := s.DistanceMeters
				nearestExpresswayDist = &dist
			}
		case "arterial":
			if nearestArterialDist == nil || s.DistanceMeters < *nearestArterialDist {
				dist := s.DistanceMeters
				nearestArterialDist = &dist
			}
		}
	}

	totalPenalty := 0.0
	var closestContributingDist *float64

	if nearestTrackDist != nil {
		if *nearestTrackDist < NoiseTrackHighImpactThresholdMeters {
			totalPenalty += NoiseTrackHighImpactPenalty
			closestContributingDist = updateMinDist(closestContributingDist, *nearestTrackDist)
		} else if *nearestTrackDist <= NoiseTrackMediumImpactThresholdMeters {
			totalPenalty += NoiseTrackMediumImpactPenalty
			closestContributingDist = updateMinDist(closestContributingDist, *nearestTrackDist)
		}
	}

	if nearestExpresswayDist != nil && *nearestExpresswayDist < NoiseExpresswayThresholdMeters {
		totalPenalty += NoiseExpresswayPenalty
		closestContributingDist = updateMinDist(closestContributingDist, *nearestExpresswayDist)
	}

	if nearestArterialDist != nil && *nearestArterialDist < NoiseArterialThresholdMeters {
		totalPenalty += NoiseArterialPenalty
		closestContributingDist = updateMinDist(closestContributingDist, *nearestArterialDist)
	}

	finalScore := math.Max(0.0, 100.0-totalPenalty)
	score = float32(math.Round(finalScore*10) / 10)

	if closestContributingDist != nil {
		distVal := float32(math.Round(*closestContributingDist*10) / 10)
		minContributingDistance = &distVal
	}

	return score, minContributingDistance
}

func updateMinDist(current *float64, candidate float64) *float64 {
	if current == nil || candidate < *current {
		return &candidate
	}
	return current
}
