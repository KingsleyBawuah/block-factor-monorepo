package scoring_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
)

func TestCalculateGreenScore_Scenarios(t *testing.T) {
	tests := []struct {
		name          string
		parks         []scoring.ParkProximity
		expectedScore float32
		hasDistance   bool
	}{
		{
			name: "Park within 0.1 mi (100m)",
			parks: []scoring.ParkProximity{
				{Name: "Pilsen Park", ParkClass: "COMMUNITY PARK", DistanceMeters: 100.0},
			},
			expectedScore: 100.0,
			hasDistance:   true,
		},
		{
			name: "Park at 321.8m (midpoint between 0.1 and 0.3 mi -> 75 pts)",
			parks: []scoring.ParkProximity{
				{Name: "Dvorak Park", ParkClass: "COMMUNITY PARK", DistanceMeters: 321.869},
			},
			expectedScore: 75.0,
			hasDistance:   true,
		},
		{
			name: "Park at 321.8m with nearby Bloomingdale Trail (+15 bonus -> 90 pts)",
			parks: []scoring.ParkProximity{
				{Name: "Dvorak Park", ParkClass: "COMMUNITY PARK", DistanceMeters: 321.869},
				{Name: "The 606 Trail", ParkClass: "LINEAR PARK", DistanceMeters: 350.0},
			},
			expectedScore: 90.0, // 75 + 15
			hasDistance:   true,
		},

		{
			name: "Park at 50m with nearby Lakefront Trail (clamped at 100)",
			parks: []scoring.ParkProximity{
				{Name: "Grant Park", ParkClass: "REGIONAL PARK", DistanceMeters: 50.0},
				{Name: "Lakefront Trail", ParkClass: "LINEAR PARK", DistanceMeters: 50.0},
			},
			expectedScore: 100.0, // 100 + 15 -> 100
			hasDistance:   true,
		},
		{
			name: "Park at 844.9m (midpoint between 0.3 and 0.75 mi -> 25 pts)",
			parks: []scoring.ParkProximity{
				{Name: "Humboldt Park", ParkClass: "REGIONAL PARK", DistanceMeters: 844.906},
			},
			expectedScore: 25.0,
			hasDistance:   true,
		},
		{
			name: "No parks within 0.75 mi",
			parks: []scoring.ParkProximity{
				{Name: "Distant Park", ParkClass: "REGIONAL PARK", DistanceMeters: 1500.0},
			},
			expectedScore: 0.0,
			hasDistance:   true,
		},
		{
			name:          "Empty parks list",
			parks:         nil,
			expectedScore: 0.0,
			hasDistance:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, dist := scoring.CalculateGreenScore(tt.parks)
			if score != tt.expectedScore {
				t.Errorf("expected score %f, got %f", tt.expectedScore, score)
			}
			if (dist != nil) != tt.hasDistance {
				t.Errorf("expected distance presence %t, got %v", tt.hasDistance, dist)
			}
		})
	}
}

type mockGreenQuerier struct {
	parksFunc func(ctx context.Context, lat, lon, maxDistance float64) ([]scoring.ParkProximity, error)
}

func (m *mockGreenQuerier) GetNearbyParks(ctx context.Context, lat, lon, maxDistance float64) ([]scoring.ParkProximity, error) {
	return m.parksFunc(ctx, lat, lon, maxDistance)
}

func TestGreenCalculator_Calculate_Success(t *testing.T) {
	querier := &mockGreenQuerier{
		parksFunc: func(_ context.Context, lat, lon, maxDistance float64) ([]scoring.ParkProximity, error) {
			if lat != 41.8781 || lon != -87.6298 {
				t.Errorf("unexpected coords (%f, %f)", lat, lon)
			}
			if maxDistance != scoring.ParkMaxQueryRadiusMeters {
				t.Errorf("expected max distance %f, got %f", scoring.ParkMaxQueryRadiusMeters, maxDistance)
			}
			return []scoring.ParkProximity{
				{Name: "Millennium Park", ParkClass: "REGIONAL PARK", DistanceMeters: 100.0},
			}, nil
		},
	}

	calc := scoring.NewGreenCalculator(querier)
	if calc.Name() != scoring.FactorGreen {
		t.Errorf("expected name %q, got %q", scoring.FactorGreen, calc.Name())
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
}

func TestGreenCalculator_Calculate_ErrorDegrades(t *testing.T) {
	querier := &mockGreenQuerier{
		parksFunc: func(_ context.Context, _, _, _ float64) ([]scoring.ParkProximity, error) {
			return nil, errors.New("database timeout")
		},
	}

	calc := scoring.NewGreenCalculator(querier)
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
