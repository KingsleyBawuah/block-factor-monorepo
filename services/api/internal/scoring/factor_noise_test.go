package scoring_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
)

func TestCalculateNoiseScore_Scenarios(t *testing.T) {
	elevated := "elevated"
	atGrade := "at_grade"
	underground := "underground"

	tests := []struct {
		name             string
		sources          []scoring.NoiseProximity
		expectedScore    float32
		expectedDistance *float32
	}{
		{
			name:             "Quiet residential street with no noise sources",
			sources:          nil,
			expectedScore:    100.0,
			expectedDistance: nil,
		},
		{
			name: "Elevated CTA track at 30m (< 150 ft high impact)",
			sources: []scoring.NoiseProximity{
				{SourceType: "cta_track", Grade: &elevated, DistanceMeters: 30.0},
			},
			expectedScore:    65.0, // 100 - 35
			expectedDistance: ptr(float32(30.0)),
		},
		{
			name: "At-grade CTA track at 60m (150-300 ft medium impact)",
			sources: []scoring.NoiseProximity{
				{SourceType: "cta_track", Grade: &atGrade, DistanceMeters: 60.0},
			},
			expectedScore:    80.0, // 100 - 20
			expectedDistance: ptr(float32(60.0)),
		},
		{
			name: "Elevated CTA track at 120m (> 300 ft no penalty)",
			sources: []scoring.NoiseProximity{
				{SourceType: "cta_track", Grade: &elevated, DistanceMeters: 120.0},
			},
			expectedScore:    100.0,
			expectedDistance: nil,
		},
		{
			name: "Underground CTA track (subway) is exempt from penalties",
			sources: []scoring.NoiseProximity{
				{SourceType: "cta_track", Grade: &underground, DistanceMeters: 20.0},
			},
			expectedScore:    100.0,
			expectedDistance: nil,
		},
		{
			name: "Expressway at 100m (< 500 ft / 152.4m)",
			sources: []scoring.NoiseProximity{
				{SourceType: "expressway", Grade: nil, DistanceMeters: 100.0},
			},
			expectedScore:    70.0, // 100 - 30
			expectedDistance: ptr(float32(100.0)),
		},
		{
			name: "4-lane arterial at 25m (< 100 ft / 30.48m)",
			sources: []scoring.NoiseProximity{
				{SourceType: "arterial", Grade: nil, DistanceMeters: 25.0},
			},
			expectedScore:    85.0, // 100 - 15
			expectedDistance: ptr(float32(25.0)),
		},
		{
			name: "Cumulative penalties: Elevated track (30m, -35) + Expressway (100m, -30) + Arterial (20m, -15)",
			sources: []scoring.NoiseProximity{
				{SourceType: "cta_track", Grade: &elevated, DistanceMeters: 30.0},
				{SourceType: "expressway", Grade: nil, DistanceMeters: 100.0},
				{SourceType: "arterial", Grade: nil, DistanceMeters: 20.0},
			},
			expectedScore:    20.0, // 100 - (35 + 30 + 15) = 20.0
			expectedDistance: ptr(float32(20.0)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, dist := scoring.CalculateNoiseScore(tt.sources)
			if score != tt.expectedScore {
				t.Errorf("expected score %f, got %f", tt.expectedScore, score)
			}
			if (dist == nil && tt.expectedDistance != nil) || (dist != nil && tt.expectedDistance == nil) {
				t.Errorf("expected distance %v, got %v", tt.expectedDistance, dist)
			} else if dist != nil && tt.expectedDistance != nil && *dist != *tt.expectedDistance {
				t.Errorf("expected distance %f, got %f", *tt.expectedDistance, *dist)
			}
		})
	}
}

type mockNoiseQuerier struct {
	sourcesFunc func(ctx context.Context, lat, lon float64, maxDistance float64) ([]scoring.NoiseProximity, error)
}

func (m *mockNoiseQuerier) GetNearbyNoiseSources(ctx context.Context, lat, lon float64, maxDistance float64) ([]scoring.NoiseProximity, error) {
	return m.sourcesFunc(ctx, lat, lon, maxDistance)
}

func TestNoiseCalculator_Calculate_Success(t *testing.T) {
	elevated := "elevated"
	querier := &mockNoiseQuerier{
		sourcesFunc: func(_ context.Context, lat, lon float64, maxDistance float64) ([]scoring.NoiseProximity, error) {
			if lat != 41.8781 || lon != -87.6298 {
				t.Errorf("unexpected coords (%f, %f)", lat, lon)
			}
			if maxDistance != scoring.NoiseMaxQueryRadiusMeters {
				t.Errorf("expected max distance %f, got %f", scoring.NoiseMaxQueryRadiusMeters, maxDistance)
			}
			return []scoring.NoiseProximity{
				{SourceType: "cta_track", Grade: &elevated, DistanceMeters: 30.0},
			}, nil
		},
	}

	calc := scoring.NewNoiseCalculator(querier)
	if calc.Name() != scoring.FactorNoise {
		t.Errorf("expected name %q, got %q", scoring.FactorNoise, calc.Name())
	}

	res, err := calc.Calculate(context.Background(), scoring.ScoreParams{
		Origin: api.Coordinates{Lat: 41.8781, Lon: -87.6298},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Degraded {
		t.Errorf("expected not degraded")
	}
	if res.Score == nil || *res.Score != 65.0 {
		t.Errorf("expected score 65.0, got %v", res.Score)
	}
	if res.DistanceMeters == nil || *res.DistanceMeters != 30.0 {
		t.Errorf("expected distance 30.0, got %v", res.DistanceMeters)
	}
}

func TestNoiseCalculator_Calculate_ErrorDegrades(t *testing.T) {
	querier := &mockNoiseQuerier{
		sourcesFunc: func(_ context.Context, _, _ float64, _ float64) ([]scoring.NoiseProximity, error) {
			return nil, errors.New("database connection broken")
		},
	}

	calc := scoring.NewNoiseCalculator(querier)
	res, err := calc.Calculate(context.Background(), scoring.ScoreParams{
		Origin: api.Coordinates{Lat: 41.8781, Lon: -87.6298},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !res.Degraded {
		t.Errorf("expected result to be degraded on error")
	}
}
