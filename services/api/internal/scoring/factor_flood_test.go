package scoring_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
)

func TestCalculateFloodScore_Formulas(t *testing.T) {
	tests := []struct {
		name      string
		zone      string
		incidents int
		expected  float32
	}{
		{
			name:      "Zone X with zero incidents",
			zone:      "X",
			incidents: 0,
			expected:  100.0,
		},
		{
			name:      "Zone X with 2 sewer backup complaints",
			zone:      "X",
			incidents: 2,
			expected:  90.0, // 100 - (2 * 5)
		},
		{
			name:      "Zone AE with zero incidents",
			zone:      "AE",
			incidents: 0,
			expected:  40.0,
		},
		{
			name:      "Zone AE with 3 incidents",
			zone:      "AE",
			incidents: 3,
			expected:  25.0, // 40 - (3 * 5)
		},
		{
			name:      "Zone A with 10 incidents floored at 0",
			zone:      "A",
			incidents: 10,
			expected:  0.0, // 40 - 50 = -10 -> 0.0
		},
		{
			name:      "Zone D with 1 incident",
			zone:      "D",
			incidents: 1,
			expected:  65.0, // 70 - 5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoring.CalculateFloodScore(tt.zone, tt.incidents)
			if score != tt.expected {
				t.Errorf("expected score %f, got %f", tt.expected, score)
			}
		})
	}
}

type mockFloodQuerier struct {
	zoneFunc  func(ctx context.Context, lat, lon float64) (string, error)
	countFunc func(ctx context.Context, lat, lon float64, radiusMeters float64, months int) (int, error)
}

func (m *mockFloodQuerier) GetFEMAFloodZone(ctx context.Context, lat, lon float64) (string, error) {
	return m.zoneFunc(ctx, lat, lon)
}

func (m *mockFloodQuerier) GetSewer311IncidentCount(ctx context.Context, lat, lon float64, radiusMeters float64, months int) (int, error) {
	return m.countFunc(ctx, lat, lon, radiusMeters, months)
}

func TestFloodCalculator_Calculate_Success(t *testing.T) {
	querier := &mockFloodQuerier{
		zoneFunc: func(_ context.Context, lat, lon float64) (string, error) {
			if lat != 41.8781 || lon != -87.6298 {
				t.Errorf("unexpected coords (%f, %f)", lat, lon)
			}
			return "X", nil
		},
		countFunc: func(_ context.Context, _ float64, _ float64, radiusMeters float64, months int) (int, error) {
			if radiusMeters != scoring.Sewer311BufferMeters {
				t.Errorf("expected radius %f, got %f", scoring.Sewer311BufferMeters, radiusMeters)
			}
			if months != scoring.Sewer311IncidentTrailingMonths {
				t.Errorf("expected months %d, got %d", scoring.Sewer311IncidentTrailingMonths, months)
			}
			return 2, nil
		},
	}

	calc := scoring.NewFloodCalculator(querier)
	if calc.Name() != scoring.FactorFlood {
		t.Errorf("expected name %q, got %q", scoring.FactorFlood, calc.Name())
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
	if res.Score == nil || *res.Score != 90.0 {
		t.Errorf("expected score 90.0, got %v", res.Score)
	}
}

func TestFloodCalculator_Calculate_ErrorDegrades(t *testing.T) {
	querier := &mockFloodQuerier{
		zoneFunc: func(_ context.Context, _, _ float64) (string, error) {
			return "", errors.New("database connection broken")
		},
		countFunc: func(_ context.Context, _, _ float64, _ float64, _ int) (int, error) {
			return 0, nil
		},
	}

	calc := scoring.NewFloodCalculator(querier)
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
