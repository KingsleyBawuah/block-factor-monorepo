package scoring_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
)

func TestCalculateBikeScore_TiersAndDecay(t *testing.T) {
	tests := []struct {
		name          string
		routes        []scoring.BikeRouteProximity
		expectedScore float32
	}{
		{
			name: "Protected bike lane within 0.1 mi (100m)",
			routes: []scoring.BikeRouteProximity{
				{RouteType: "Protected Bike Lane", DistanceMeters: 100.0, AzimuthDegrees: ptr(0.0)},
			},
			expectedScore: 100.0,
		},
		{
			name: "Buffered bike lane within 0.1 mi (100m)",
			routes: []scoring.BikeRouteProximity{
				{RouteType: "Buffered Bike Lane", DistanceMeters: 100.0, AzimuthDegrees: ptr(0.0)},
			},
			expectedScore: 70.0,
		},
		{
			name: "Painted bike lane within 0.1 mi (100m)",
			routes: []scoring.BikeRouteProximity{
				{RouteType: "Bike Lane", DistanceMeters: 100.0, AzimuthDegrees: ptr(0.0)},
			},
			expectedScore: 40.0,
		},
		{
			name: "Marked shared lane within 0.1 mi (100m)",
			routes: []scoring.BikeRouteProximity{
				{RouteType: "Marked Shared Lane", DistanceMeters: 100.0, AzimuthDegrees: ptr(0.0)},
			},
			expectedScore: 15.0,
		},
		{
			name: "High quality protected lane at 400m outscores close shared lane at 50m",
			routes: []scoring.BikeRouteProximity{
				{RouteType: "Marked Shared Lane", DistanceMeters: 50.0, AzimuthDegrees: ptr(90.0)},
				{RouteType: "Protected Bike Lane", DistanceMeters: 400.0, AzimuthDegrees: ptr(90.0)},
			},
			// Protected lane: 100 * (804.672 - 400) / (804.672 - 160.934) = 62.86 -> 62.9
			expectedScore: 62.9,
		},
		{
			name: "Dual-axis network bonus (+15 pts for N-S and E-W buffered lanes)",
			routes: []scoring.BikeRouteProximity{
				{RouteType: "Buffered Bike Lane", DistanceMeters: 100.0, AzimuthDegrees: ptr(0.0)},  // North-South (70 pts)
				{RouteType: "Buffered Bike Lane", DistanceMeters: 200.0, AzimuthDegrees: ptr(90.0)}, // East-West
			},
			expectedScore: 85.0, // 70 + 15
		},
		{
			name: "Protected lane with dual axis bonus clamped at 100",
			routes: []scoring.BikeRouteProximity{
				{RouteType: "Protected Bike Lane", DistanceMeters: 100.0, AzimuthDegrees: ptr(0.0)},
				{RouteType: "Protected Bike Lane", DistanceMeters: 100.0, AzimuthDegrees: ptr(90.0)},
			},
			expectedScore: 100.0, // 100 + 15 -> clamped to 100
		},
		{
			name:          "No bike infrastructure within 0.5 mi",
			routes:        nil,
			expectedScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _ := scoring.CalculateBikeScore(tt.routes)
			if score != tt.expectedScore {
				t.Errorf("expected score %f, got %f", tt.expectedScore, score)
			}
		})
	}
}

func TestCalculateBikeScore_MinDistanceMatchesBestSegment(t *testing.T) {
	// The 400m protected lane produces the winning score, so the returned
	// distance must describe that segment, not the closer but lower-scoring
	// shared lane at 50m.
	routes := []scoring.BikeRouteProximity{
		{RouteType: "Marked Shared Lane", DistanceMeters: 50.0, AzimuthDegrees: ptr(90.0)},
		{RouteType: "Protected Bike Lane", DistanceMeters: 400.0, AzimuthDegrees: ptr(90.0)},
	}

	_, minDistance := scoring.CalculateBikeScore(routes)
	if minDistance == nil || *minDistance != 400.0 {
		t.Errorf("expected minDistance 400.0, got %v", minDistance)
	}
}

func TestCalculateBikeScore_NilAzimuthExcludedFromNetworkBonus(t *testing.T) {
	// A degenerate (loop) segment has no defined bearing and must not count
	// toward either the North-South or East-West side of the network bonus.
	routes := []scoring.BikeRouteProximity{
		{RouteType: "Buffered Bike Lane", DistanceMeters: 100.0, AzimuthDegrees: nil},
		{RouteType: "Buffered Bike Lane", DistanceMeters: 200.0, AzimuthDegrees: ptr(90.0)},
	}

	score, _ := scoring.CalculateBikeScore(routes)
	if score != 70.0 {
		t.Errorf("expected score 70.0 (no network bonus), got %f", score)
	}
}

type mockBikeQuerier struct {
	routesFunc func(ctx context.Context, lat, lon, maxDistance float64) ([]scoring.BikeRouteProximity, error)
}

func (m *mockBikeQuerier) GetNearbyBikeRoutes(ctx context.Context, lat, lon, maxDistance float64) ([]scoring.BikeRouteProximity, error) {
	return m.routesFunc(ctx, lat, lon, maxDistance)
}

func TestBikeCalculator_Calculate_Success(t *testing.T) {
	querier := &mockBikeQuerier{
		routesFunc: func(_ context.Context, lat, lon, maxDistance float64) ([]scoring.BikeRouteProximity, error) {
			if lat != 41.8781 || lon != -87.6298 {
				t.Errorf("unexpected coords (%f, %f)", lat, lon)
			}
			if maxDistance != scoring.BikeMaxQueryRadiusMeters {
				t.Errorf("expected max distance %f, got %f", scoring.BikeMaxQueryRadiusMeters, maxDistance)
			}
			return []scoring.BikeRouteProximity{
				{RouteType: "Protected Bike Lane", DistanceMeters: 100.0, AzimuthDegrees: ptr(0.0)},
			}, nil
		},
	}

	calc := scoring.NewBikeCalculator(querier)
	if calc.Name() != scoring.FactorBike {
		t.Errorf("expected name %q, got %q", scoring.FactorBike, calc.Name())
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

func TestBikeCalculator_Calculate_ErrorDegrades(t *testing.T) {
	querier := &mockBikeQuerier{
		routesFunc: func(_ context.Context, _, _, _ float64) ([]scoring.BikeRouteProximity, error) {
			return nil, errors.New("database connection failed")
		},
	}

	calc := scoring.NewBikeCalculator(querier)
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
