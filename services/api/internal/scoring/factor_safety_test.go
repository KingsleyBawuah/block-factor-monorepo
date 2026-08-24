package scoring_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
)

func TestCalculateSafetyScore_Percentiles(t *testing.T) {
	tests := []struct {
		name          string
		percentile    float64
		expectedScore float32
	}{
		{
			name:          "0th percentile crime (lowest in city)",
			percentile:    0.0,
			expectedScore: 100.0,
		},
		{
			name:          "15th percentile crime",
			percentile:    15.0,
			expectedScore: 85.0,
		},
		{
			name:          "50th percentile crime (city median)",
			percentile:    50.0,
			expectedScore: 50.0,
		},
		{
			name:          "87.5 percentile crime",
			percentile:    87.5,
			expectedScore: 12.5,
		},
		{
			name:          "100th percentile crime (highest in city)",
			percentile:    100.0,
			expectedScore: 0.0,
		},
		{
			name:          "Negative percentile clamped to 100",
			percentile:    -5.0,
			expectedScore: 100.0,
		},
		{
			name:          "Over-100 percentile clamped to 0",
			percentile:    105.0,
			expectedScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoring.CalculateSafetyScore(tt.percentile)
			if score != tt.expectedScore {
				t.Errorf("expected score %f, got %f", tt.expectedScore, score)
			}
		})
	}
}

type mockSafetyQuerier struct {
	percentileFunc func(ctx context.Context, lat, lon float64) (float64, error)
}

func (m *mockSafetyQuerier) GetSafetyPercentile(ctx context.Context, lat, lon float64) (float64, error) {
	return m.percentileFunc(ctx, lat, lon)
}

func TestSafetyCalculator_Calculate_Success(t *testing.T) {
	querier := &mockSafetyQuerier{
		percentileFunc: func(_ context.Context, lat, lon float64) (float64, error) {
			if lat != 41.8781 || lon != -87.6298 {
				t.Errorf("unexpected coords (%f, %f)", lat, lon)
			}
			return 20.0, nil
		},
	}

	calc := scoring.NewSafetyCalculator(querier)
	if calc.Name() != scoring.FactorSafety {
		t.Errorf("expected name %q, got %q", scoring.FactorSafety, calc.Name())
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
	if res.Score == nil || *res.Score != 80.0 {
		t.Errorf("expected score 80.0, got %v", res.Score)
	}
}

func TestSafetyCalculator_Calculate_ErrorDegrades(t *testing.T) {
	querier := &mockSafetyQuerier{
		percentileFunc: func(_ context.Context, _, _ float64) (float64, error) {
			return 0, errors.New("database timeout")
		},
	}

	calc := scoring.NewSafetyCalculator(querier)
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
