package scoring_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
)

func TestCalculateGroceryScore_Scenarios(t *testing.T) {
	tests := []struct {
		name          string
		pois          []scoring.OsmPoi
		expectedScore float32
		expectedPois  int
	}{
		{
			name: "Supermarket within 0.3 mi (200m)",
			pois: []scoring.OsmPoi{
				{Name: "Trader Joe's", PoiType: "supermarket", DistanceMeters: 200.0},
			},
			expectedScore: 100.0,
			expectedPois:  1,
		},
		{
			name: "Supermarket at 800m (0.3 - 0.75 mi) without pharmacy",
			pois: []scoring.OsmPoi{
				{Name: "Jewel-Osco", PoiType: "supermarket", DistanceMeters: 800.0},
			},
			expectedScore: 75.0,
			expectedPois:  1,
		},
		{
			name: "Supermarket at 800m with pharmacy at 250m (+10 boost)",
			pois: []scoring.OsmPoi{
				{Name: "Jewel-Osco", PoiType: "supermarket", DistanceMeters: 800.0},
				{Name: "Walgreens", PoiType: "pharmacy", DistanceMeters: 250.0},
			},
			expectedScore: 85.0, // 75 + 10
			expectedPois:  2,
		},
		{
			name: "Supermarket at 200m with pharmacy at 100m (clamped at 100)",
			pois: []scoring.OsmPoi{
				{Name: "Mariano's", PoiType: "supermarket", DistanceMeters: 200.0},
				{Name: "CVS", PoiType: "pharmacy", DistanceMeters: 100.0},
			},
			expectedScore: 100.0, // 100 + 10 -> 100
			expectedPois:  2,
		},
		{
			name: "Corner store fallback within 0.3 mi",
			pois: []scoring.OsmPoi{
				{Name: "Local Bodega", PoiType: "convenience", DistanceMeters: 300.0},
			},
			expectedScore: 40.0,
			expectedPois:  1,
		},
		{
			name: "Corner store with nearby pharmacy",
			pois: []scoring.OsmPoi{
				{Name: "Local Bodega", PoiType: "convenience", DistanceMeters: 300.0},
				{Name: "Walgreens", PoiType: "pharmacy", DistanceMeters: 200.0},
			},
			expectedScore: 50.0, // 40 + 10
			expectedPois:  2,
		},
		{
			name:          "No retail food POIs within 1.0 mi",
			pois:          nil,
			expectedScore: 0.0,
			expectedPois:  0,
		},
		{
			name: "Close corner store outscores a heavily-decayed distant supermarket",
			pois: []scoring.OsmPoi{
				{Name: "Distant Mariano's", PoiType: "supermarket", DistanceMeters: 1600.0},
				{Name: "Local Bodega", PoiType: "convenience", DistanceMeters: 50.0},
			},
			expectedScore: 40.0,
			expectedPois:  1,
		},
		{
			name: "Convenience store beyond its threshold contributes nothing",
			pois: []scoring.OsmPoi{
				{Name: "Far Bodega", PoiType: "convenience", DistanceMeters: 900.0},
			},
			expectedScore: 0.0,
			expectedPois:  0,
		},
		{
			name: "Pharmacy beyond its threshold contributes nothing",
			pois: []scoring.OsmPoi{
				{Name: "Far Walgreens", PoiType: "pharmacy", DistanceMeters: 500.0},
			},
			expectedScore: 0.0,
			expectedPois:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _, contributing := scoring.CalculateGroceryScore(tt.pois)
			if score != tt.expectedScore {
				t.Errorf("expected score %f, got %f", tt.expectedScore, score)
			}
			if len(contributing) != tt.expectedPois {
				t.Errorf("expected %d contributing pois, got %d", tt.expectedPois, len(contributing))
			}
		})
	}
}

func TestCalculateGroceryScore_PharmacyOnlyReturnsItsDistance(t *testing.T) {
	// A pharmacy-only result still produces a nonzero score via the boost,
	// so minDistance must reflect the pharmacy's distance rather than nil.
	pois := []scoring.OsmPoi{
		{Name: "Walgreens", PoiType: "pharmacy", DistanceMeters: 100.0},
	}

	score, minDistance, contributing := scoring.CalculateGroceryScore(pois)
	if score != 10.0 {
		t.Errorf("expected score 10.0, got %f", score)
	}
	if minDistance == nil || *minDistance != 100.0 {
		t.Errorf("expected minDistance 100.0, got %v", minDistance)
	}
	if len(contributing) != 1 {
		t.Errorf("expected 1 contributing poi, got %d", len(contributing))
	}
}

type mockGroceryQuerier struct {
	poisFunc func(ctx context.Context, lat, lon, maxDistance float64) ([]scoring.OsmPoi, error)
}

func (m *mockGroceryQuerier) GetNearbyGroceryPois(ctx context.Context, lat, lon, maxDistance float64) ([]scoring.OsmPoi, error) {
	return m.poisFunc(ctx, lat, lon, maxDistance)
}

func TestGroceryCalculator_Calculate_Success(t *testing.T) {
	querier := &mockGroceryQuerier{
		poisFunc: func(_ context.Context, lat, lon, maxDistance float64) ([]scoring.OsmPoi, error) {
			if lat != 41.8781 || lon != -87.6298 {
				t.Errorf("unexpected coords (%f, %f)", lat, lon)
			}
			if maxDistance != scoring.GroceryMaxQueryRadiusMeters {
				t.Errorf("expected max distance %f, got %f", scoring.GroceryMaxQueryRadiusMeters, maxDistance)
			}
			return []scoring.OsmPoi{
				{Name: "Aldi", PoiType: "supermarket", DistanceMeters: 300.0},
			}, nil
		},
	}

	calc := scoring.NewGroceryCalculator(querier)
	if calc.Name() != scoring.FactorGrocery {
		t.Errorf("expected name %q, got %q", scoring.FactorGrocery, calc.Name())
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
	if res.Score == nil || *res.Score != 100.0 {
		t.Errorf("expected score 100.0, got %v", res.Score)
	}
	if len(res.ContributingPois) != 1 {
		t.Errorf("expected 1 contributing poi, got %d", len(res.ContributingPois))
	}
}

func TestGroceryCalculator_Calculate_ErrorDegrades(t *testing.T) {
	querier := &mockGroceryQuerier{
		poisFunc: func(_ context.Context, _, _, _ float64) ([]scoring.OsmPoi, error) {
			return nil, errors.New("database timeout")
		},
	}

	calc := scoring.NewGroceryCalculator(querier)
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
