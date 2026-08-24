package scoring

import (
	"context"
	"errors"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
)

type mockTransitQuerier struct {
	stations             []CtaStation
	corridors            []BusCorridor
	lineServices         []CtaLineService
	err                  error
	getLineServiceCalled bool
}

func (m *mockTransitQuerier) GetNearbyCtaStations(_ context.Context, _, _ float64, _ float64) ([]CtaStation, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.stations, nil
}

func (m *mockTransitQuerier) GetNearbyBusCorridors(_ context.Context, _, _ float64, _, _ float64) ([]BusCorridor, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.corridors, nil
}

func (m *mockTransitQuerier) GetLineService(_ context.Context) ([]CtaLineService, error) {
	m.getLineServiceCalled = true
	if m.err != nil {
		return nil, m.err
	}
	return m.lineServices, nil
}

func TestCalculateTransitScore_DistanceBands(t *testing.T) {
	tests := []struct {
		name         string
		stations     []CtaStation
		corridors    []BusCorridor
		wantScore    float32
		wantDistance *float32
		wantPois     int
	}{
		{
			name:         "No stations and no bus corridors",
			stations:     nil,
			corridors:    nil,
			wantScore:    0.0,
			wantDistance: nil,
			wantPois:     0,
		},
		{
			name: "Single-line station <= 0.25 mi (100 pts)",
			stations: []CtaStation{
				{
					StopName:       "18th",
					Lines:          []string{"Pink"},
					DistanceMeters: 200.0,
				},
			},
			corridors:    nil,
			wantScore:    100.0,
			wantDistance: float32Ptr(200.0),
			wantPois:     1,
		},
		{
			name: "Single-line station exactly at 0.25 mi boundary (100 pts)",
			stations: []CtaStation{
				{
					StopName:       "Damen",
					Lines:          []string{"Blue"},
					DistanceMeters: TransitStationCloseThresholdMeters,
				},
			},
			corridors:    nil,
			wantScore:    100.0,
			wantDistance: float32Ptr(402.3),
			wantPois:     1,
		},
		{
			name: "Single-line station at midpoint (~0.625 mi -> 50 pts)",
			stations: []CtaStation{
				{
					StopName:       "Western",
					Lines:          []string{"Orange"},
					DistanceMeters: (TransitStationCloseThresholdMeters + TransitMaxQueryRadiusMeters) / 2.0, // 1005.84 m
				},
			},
			corridors:    nil,
			wantScore:    50.0,
			wantDistance: float32Ptr(1005.8),
			wantPois:     1,
		},
		{
			name: "Single-line station at 1.0 mi boundary (0 pts)",
			stations: []CtaStation{
				{
					StopName:       "Kedzie",
					Lines:          []string{"Brown"},
					DistanceMeters: TransitMaxQueryRadiusMeters,
				},
			},
			corridors:    nil,
			wantScore:    0.0,
			wantDistance: float32Ptr(1609.3),
			wantPois:     1,
		},
		{
			name: "Single-line station beyond 1.0 mi (> 1609.3 m -> 0 pts)",
			stations: []CtaStation{
				{
					StopName:       "Kimball",
					Lines:          []string{"Brown"},
					DistanceMeters: 1800.0,
				},
			},
			corridors:    nil,
			wantScore:    0.0,
			wantDistance: float32Ptr(1800.0),
			wantPois:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, minDistance, pois := CalculateTransitScore(tt.stations, tt.corridors, nil)
			if score != tt.wantScore {
				t.Errorf("got score %v, want %v", score, tt.wantScore)
			}
			if (minDistance == nil) != (tt.wantDistance == nil) {
				t.Fatalf("got minDistance %v, want %v", minDistance, tt.wantDistance)
			}
			if minDistance != nil && *minDistance != *tt.wantDistance {
				t.Errorf("got minDistance %v, want %v", *minDistance, *tt.wantDistance)
			}
			if len(pois) != tt.wantPois {
				t.Errorf("got %d contributing pois, want %d", len(pois), tt.wantPois)
			}
		})
	}
}

func TestCalculateTransitScore_DualLineBonus(t *testing.T) {
	tests := []struct {
		name      string
		stations  []CtaStation
		corridors []BusCorridor
		wantScore float32
	}{
		{
			name: "Multi-line station at 0.625 mi (base 50 pts + 10 dual-line bonus = 60 pts)",
			stations: []CtaStation{
				{
					StopName:       "Belmont",
					Lines:          []string{"Red", "Brown", "Purple"},
					DistanceMeters: 1005.84, // Midpoint decay = 50.0 pts
				},
			},
			corridors: nil,
			wantScore: 60.0,
		},
		{
			name: "Two separate single-line stations within range (Red + Blue -> 10 bonus)",
			stations: []CtaStation{
				{
					StopName:       "Grand",
					Lines:          []string{"Red"},
					DistanceMeters: 1005.84, // 50 pts base
				},
				{
					StopName:       "Grand",
					Lines:          []string{"Blue"},
					DistanceMeters: 1200.0,
				},
			},
			corridors: nil,
			wantScore: 60.0,
		},
		{
			name: "Multi-line station close <= 0.25 mi (base 100 + 10 bonus clamped to 100)",
			stations: []CtaStation{
				{
					StopName:       "Fullerton",
					Lines:          []string{"Red", "Brown", "Purple"},
					DistanceMeters: 250.0,
				},
			},
			corridors: nil,
			wantScore: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _, _ := CalculateTransitScore(tt.stations, tt.corridors, nil)
			if score != tt.wantScore {
				t.Errorf("got score %v, want %v", score, tt.wantScore)
			}
		})
	}
}

func TestCalculateTransitScore_BusCorridorBonus(t *testing.T) {
	tests := []struct {
		name      string
		stations  []CtaStation
		corridors []BusCorridor
		wantScore float32
		wantPois  int
	}{
		{
			name:     "High-frequency bus only, no 'L' stop (base 0 + 10 bonus = 10 pts)",
			stations: nil,
			corridors: []BusCorridor{
				{
					RouteID:            "22",
					RouteName:          "Clark",
					MeanHeadwayMinutes: 7.5,
					DistanceMeters:     150.0,
				},
			},
			wantScore: 10.0,
			wantPois:  1,
		},
		{
			name: "Single-line station at 0.625 mi (50 pts) + high-frequency bus (10 pts) = 60 pts",
			stations: []CtaStation{
				{
					StopName:       "California",
					Lines:          []string{"Blue"},
					DistanceMeters: 1005.84,
				},
			},
			corridors: []BusCorridor{
				{
					RouteID:            "66",
					RouteName:          "Chicago",
					MeanHeadwayMinutes: 6.2,
					DistanceMeters:     200.0,
				},
			},
			wantScore: 60.0,
			wantPois:  2,
		},
		{
			name: "Bus corridor with headway > 10 min gets no bonus",
			stations: []CtaStation{
				{
					StopName:       "California",
					Lines:          []string{"Blue"},
					DistanceMeters: 1005.84,
				},
			},
			corridors: []BusCorridor{
				{
					RouteID:            "126",
					RouteName:          "Jackson",
					MeanHeadwayMinutes: 15.0,
					DistanceMeters:     100.0,
				},
			},
			wantScore: 50.0,
			wantPois:  1,
		},
		{
			name: "Bus corridor > 0.2 mi (321.87 m) gets no bonus",
			stations: []CtaStation{
				{
					StopName:       "California",
					Lines:          []string{"Blue"},
					DistanceMeters: 1005.84,
				},
			},
			corridors: []BusCorridor{
				{
					RouteID:            "66",
					RouteName:          "Chicago",
					MeanHeadwayMinutes: 6.0,
					DistanceMeters:     350.0,
				},
			},
			wantScore: 50.0,
			wantPois:  1,
		},
		{
			name: "Dual-line and bus corridor bonuses are independent and stack (50 base + 10 dual-line + 10 bus = 70)",
			stations: []CtaStation{
				{
					StopName:       "Belmont",
					Lines:          []string{"Red", "Brown"},
					DistanceMeters: 1005.84, // 50 pts
				},
			},
			corridors: []BusCorridor{
				{
					RouteID:            "77",
					RouteName:          "Belmont",
					MeanHeadwayMinutes: 5.5,
					DistanceMeters:     100.0,
				},
			},
			wantScore: 70.0,
			wantPois:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _, pois := CalculateTransitScore(tt.stations, tt.corridors, nil)
			if score != tt.wantScore {
				t.Errorf("got score %v, want %v", score, tt.wantScore)
			}
			if len(pois) != tt.wantPois {
				t.Errorf("got %d pois, want %d", len(pois), tt.wantPois)
			}
		})
	}
}

func TestCalculateTransitScore_BusDirectionalBonus(t *testing.T) {
	tests := []struct {
		name      string
		corridors []BusCorridor
		wantScore float32 // no stations in any case here, so score == bonus alone
	}{
		{
			name:      "No qualifying corridors -> 0 bonus",
			corridors: nil,
			wantScore: 0.0,
		},
		{
			name: "One N-S corridor only -> +10",
			corridors: []BusCorridor{
				{RouteID: "8", RouteName: "Halsted", MeanHeadwayMinutes: 5.0, DistanceMeters: 100.0, AzimuthDegrees: floatPtr(0.0)},
			},
			wantScore: 10.0,
		},
		{
			name: "One E-W corridor only -> +10",
			corridors: []BusCorridor{
				{RouteID: "66", RouteName: "Chicago", MeanHeadwayMinutes: 5.0, DistanceMeters: 100.0, AzimuthDegrees: floatPtr(90.0)},
			},
			wantScore: 10.0,
		},
		{
			name: "N-S and E-W corridors both present -> +15",
			corridors: []BusCorridor{
				{RouteID: "8", RouteName: "Halsted", MeanHeadwayMinutes: 5.0, DistanceMeters: 100.0, AzimuthDegrees: floatPtr(0.0)},
				{RouteID: "66", RouteName: "Chicago", MeanHeadwayMinutes: 5.0, DistanceMeters: 150.0, AzimuthDegrees: floatPtr(90.0)},
			},
			wantScore: 15.0,
		},
		{
			name: "Two corridors on the same axis -> still +10, not +15",
			corridors: []BusCorridor{
				{RouteID: "8", RouteName: "Halsted", MeanHeadwayMinutes: 5.0, DistanceMeters: 100.0, AzimuthDegrees: floatPtr(0.0)},
				{RouteID: "9", RouteName: "Ashland", MeanHeadwayMinutes: 5.0, DistanceMeters: 150.0, AzimuthDegrees: floatPtr(180.0)},
			},
			wantScore: 10.0,
		},
		{
			name: "Qualifying corridor with undefined bearing (loop) -> +10, no axis established",
			corridors: []BusCorridor{
				{RouteID: "20", RouteName: "Loop Shuttle", MeanHeadwayMinutes: 5.0, DistanceMeters: 100.0, AzimuthDegrees: nil},
			},
			wantScore: 10.0,
		},
		{
			name: "Corridor beyond the bonus radius doesn't count toward directional coverage",
			corridors: []BusCorridor{
				{RouteID: "8", RouteName: "Halsted", MeanHeadwayMinutes: 5.0, DistanceMeters: 100.0, AzimuthDegrees: floatPtr(0.0)},
				{RouteID: "66", RouteName: "Chicago", MeanHeadwayMinutes: 5.0, DistanceMeters: 400.0, AzimuthDegrees: floatPtr(90.0)},
			},
			wantScore: 10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _, _ := CalculateTransitScore(nil, tt.corridors, nil)
			if score != tt.wantScore {
				t.Errorf("got score %v, want %v", score, tt.wantScore)
			}
		})
	}
}

func TestCalculateTransitScore_RailQualityBonus(t *testing.T) {
	tests := []struct {
		name         string
		stations     []CtaStation
		lineServices []CtaLineService
		wantScore    float32 // no bus corridors in any case here
	}{
		{
			name: "High-frequency AM peak and overnight -> full +20 rail quality (base 100 clamps total)",
			stations: []CtaStation{
				{StopName: "Howard", Lines: []string{"Red"}, DistanceMeters: 100.0}, // base 100
			},
			lineServices: []CtaLineService{
				{LineName: "Red", AMPeakHeadwayMinutes: 4.0, OvernightHeadwayMinutes: floatPtr(10.0)},
			},
			wantScore: 100.0,
		},
		{
			name: "High-frequency AM peak and overnight, base far enough to see the bonus (50 + 20 = 70)",
			stations: []CtaStation{
				{StopName: "Howard", Lines: []string{"Red"}, DistanceMeters: 1005.84}, // base 50
			},
			lineServices: []CtaLineService{
				{LineName: "Red", AMPeakHeadwayMinutes: 4.0, OvernightHeadwayMinutes: floatPtr(10.0)},
			},
			wantScore: 70.0,
		},
		{
			name: "AM peak at the far decay threshold (20 min) contributes 0",
			stations: []CtaStation{
				{StopName: "Skokie", Lines: []string{"Yellow"}, DistanceMeters: 1005.84}, // base 50
			},
			lineServices: []CtaLineService{
				{LineName: "Yellow", AMPeakHeadwayMinutes: 20.0, OvernightHeadwayMinutes: nil},
			},
			wantScore: 50.0,
		},
		{
			name: "No scheduled overnight service (nil) contributes 0 to the overnight component",
			stations: []CtaStation{
				{StopName: "Skokie", Lines: []string{"Yellow"}, DistanceMeters: 1005.84}, // base 50
			},
			lineServices: []CtaLineService{
				{LineName: "Yellow", AMPeakHeadwayMinutes: 6.0, OvernightHeadwayMinutes: nil},
			},
			wantScore: 60.0, // 50 base + 10 AM-peak component + 0 overnight
		},
		{
			name: "Multi-line station takes the best-scoring line, not an average",
			stations: []CtaStation{
				{StopName: "Belmont", Lines: []string{"Brown", "Red"}, DistanceMeters: 1005.84}, // base 50, also +10 dual-line
			},
			lineServices: []CtaLineService{
				{LineName: "Brown", AMPeakHeadwayMinutes: 20.0, OvernightHeadwayMinutes: nil},         // 0 rail quality
				{LineName: "Red", AMPeakHeadwayMinutes: 4.0, OvernightHeadwayMinutes: floatPtr(10.0)}, // 20 rail quality
			},
			wantScore: 80.0, // 50 base + 10 dual-line + 20 rail quality (Red, not an average with Brown)
		},
		{
			name: "Line present at station but missing from cta_line_service data contributes 0",
			stations: []CtaStation{
				{StopName: "Test", Lines: []string{"Green"}, DistanceMeters: 1005.84}, // base 50
			},
			lineServices: []CtaLineService{
				{LineName: "Red", AMPeakHeadwayMinutes: 4.0, OvernightHeadwayMinutes: floatPtr(10.0)},
			},
			wantScore: 50.0,
		},
		{
			name: "Overnight headway at or beyond the far decay threshold (30 min) contributes 0, distinct from nil",
			stations: []CtaStation{
				{StopName: "Test", Lines: []string{"Green"}, DistanceMeters: 1005.84}, // base 50
			},
			lineServices: []CtaLineService{
				{LineName: "Green", AMPeakHeadwayMinutes: 20.0, OvernightHeadwayMinutes: floatPtr(30.0)},
			},
			wantScore: 50.0,
		},
		{
			name: "Headways strictly between thresholds land on the linear decay midpoint",
			stations: []CtaStation{
				{StopName: "Test", Lines: []string{"Purple"}, DistanceMeters: 1005.84}, // base 50
			},
			lineServices: []CtaLineService{
				// AM peak 13.0 = midpoint of [6,20] -> 5.0 pts; overnight 22.5 = midpoint of [15,30] -> 5.0 pts
				{LineName: "Purple", AMPeakHeadwayMinutes: 13.0, OvernightHeadwayMinutes: floatPtr(22.5)},
			},
			wantScore: 60.0, // 50 base + 5.0 + 5.0 rail quality
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _, _ := CalculateTransitScore(tt.stations, nil, tt.lineServices)
			if score != tt.wantScore {
				t.Errorf("got score %v, want %v", score, tt.wantScore)
			}
		})
	}
}

func TestTransitCalculator_Calculate_Success(t *testing.T) {
	querier := &mockTransitQuerier{
		stations: []CtaStation{
			{
				StopName:       "Clark/Lake",
				Lines:          []string{"Blue", "Green", "Brown", "Orange", "Pink", "Purple"},
				DistanceMeters: 150.0,
			},
		},
		corridors: []BusCorridor{
			{
				RouteID:            "156",
				RouteName:          "LaSalle",
				MeanHeadwayMinutes: 8.0,
				DistanceMeters:     80.0,
			},
		},
	}

	calc := NewTransitCalculator(querier)
	if calc.Name() != FactorTransit {
		t.Fatalf("got Name %v, want %v", calc.Name(), FactorTransit)
	}

	params := ScoreParams{
		Origin: api.Coordinates{Lat: 41.8857, Lon: -87.6308},
	}

	result, err := calc.Calculate(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Degraded {
		t.Errorf("expected not degraded")
	}
	if result.Score == nil || *result.Score != 100.0 {
		t.Errorf("got score %v, want 100.0", result.Score)
	}
	if len(result.ContributingPois) != 2 {
		t.Errorf("got %d contributing pois, want 2", len(result.ContributingPois))
	}
}

// TestTransitCalculator_Calculate_UnsaturatedScore exercises Calculate's full
// wiring (including GetLineService) with an unsaturated result, since
// TestTransitCalculator_Calculate_Success's 100-pt clamp can't distinguish
// correctly-wired lineServices from a bug that silently dropped them.
func TestTransitCalculator_Calculate_UnsaturatedScore(t *testing.T) {
	querier := &mockTransitQuerier{
		stations: []CtaStation{
			{
				StopName:       "Damen",
				Lines:          []string{"Blue", "Pink"},
				DistanceMeters: 1005.84, // base 50
			},
		},
		corridors: []BusCorridor{
			{
				RouteID:            "50",
				RouteName:          "Damen",
				MeanHeadwayMinutes: 6.0,
				DistanceMeters:     100.0,
				AzimuthDegrees:     floatPtr(0.0), // single axis -> +10
			},
		},
		lineServices: []CtaLineService{
			{LineName: "Blue", AMPeakHeadwayMinutes: 6.0, OvernightHeadwayMinutes: floatPtr(22.5)}, // 10 + 5 = 15
			{LineName: "Pink", AMPeakHeadwayMinutes: 20.0, OvernightHeadwayMinutes: nil},           // 0
		},
	}

	calc := NewTransitCalculator(querier)
	params := ScoreParams{Origin: api.Coordinates{Lat: 41.8857, Lon: -87.6308}}

	result, err := calc.Calculate(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !querier.getLineServiceCalled {
		t.Errorf("expected GetLineService to be called when stations are reachable")
	}

	const wantScore = 85.0 // 50 base + 10 dual-line + 10 bus + 15 rail quality (Blue, not Pink)
	if result.Score == nil || *result.Score != wantScore {
		t.Errorf("got score %v, want %v", result.Score, wantScore)
	}
}

// TestTransitCalculator_Calculate_NoStationsSkipsLineServiceQuery verifies the
// short-circuit: when no CTA stations are reachable, the rail quality bonus is
// unconditionally 0, so GetLineService should never be called.
func TestTransitCalculator_Calculate_NoStationsSkipsLineServiceQuery(t *testing.T) {
	querier := &mockTransitQuerier{
		stations: nil,
		corridors: []BusCorridor{
			{RouteID: "8", RouteName: "Halsted", MeanHeadwayMinutes: 5.0, DistanceMeters: 100.0},
		},
	}

	calc := NewTransitCalculator(querier)
	params := ScoreParams{Origin: api.Coordinates{Lat: 41.8857, Lon: -87.6308}}

	if _, err := calc.Calculate(context.Background(), params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if querier.getLineServiceCalled {
		t.Errorf("expected GetLineService NOT to be called when no stations are reachable")
	}
}

func TestTransitCalculator_Calculate_ErrorDegrades(t *testing.T) {
	querier := &mockTransitQuerier{
		err: errors.New("database connection failed"),
	}

	calc := NewTransitCalculator(querier)
	params := ScoreParams{
		Origin: api.Coordinates{Lat: 41.8857, Lon: -87.6308},
	}

	result, err := calc.Calculate(context.Background(), params)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
	if !result.Degraded {
		t.Errorf("expected Degraded=true")
	}
	if result.Score != nil {
		t.Errorf("expected nil Score on degradation, got %v", *result.Score)
	}
}

func TestTransitCalculator_Calculate_NilQuerier(t *testing.T) {
	calc := NewTransitCalculator(nil)
	params := ScoreParams{
		Origin: api.Coordinates{Lat: 41.8857, Lon: -87.6308},
	}

	result, err := calc.Calculate(context.Background(), params)
	if err == nil {
		t.Errorf("expected error for nil querier, got nil")
	}
	if !result.Degraded {
		t.Errorf("expected Degraded=true for nil querier")
	}
}

func float32Ptr(v float32) *float32 {
	return &v
}

func floatPtr(v float64) *float64 {
	return &v
}
