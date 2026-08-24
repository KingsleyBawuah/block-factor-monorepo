package scoring_test

import (
	"context"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
)

func TestCalculateSolarScore_NilUnit(t *testing.T) {
	score := scoring.CalculateSolarScore(nil)
	if score != 100.0 {
		t.Errorf("expected default score 100.0 for nil unit metadata, got %f", score)
	}
}

func TestCalculateSolarScore_Orientations(t *testing.T) {
	tests := []struct {
		name     string
		unit     *api.UnitMetadata
		expected float32
	}{
		{
			name: "South facing, street exposure, floor 1",
			unit: &api.UnitMetadata{
				Facing:   ptr(api.S),
				Exposure: ptr(api.Street),
				Floor:    ptr(1),
			},
			expected: 100.0,
		},
		{
			name: "North facing, street exposure, floor 1",
			unit: &api.UnitMetadata{
				Facing:   ptr(api.N),
				Exposure: ptr(api.Street),
				Floor:    ptr(1),
			},
			expected: 40.0,
		},
		{
			name: "East facing, street exposure, floor 1",
			unit: &api.UnitMetadata{
				Facing:   ptr(api.E),
				Exposure: ptr(api.Street),
				Floor:    ptr(1),
			},
			expected: 85.0,
		},
		{
			name: "West facing, street exposure, floor 1",
			unit: &api.UnitMetadata{
				Facing:   ptr(api.W),
				Exposure: ptr(api.Street),
				Floor:    ptr(1),
			},
			expected: 75.0,
		},
		{
			name: "Southeast facing, street exposure, floor 1",
			unit: &api.UnitMetadata{
				Facing:   ptr(api.SE),
				Exposure: ptr(api.Street),
				Floor:    ptr(1),
			},
			expected: 92.5,
		},
		{
			name: "Southwest facing, street exposure, floor 1",
			unit: &api.UnitMetadata{
				Facing:   ptr(api.SW),
				Exposure: ptr(api.Street),
				Floor:    ptr(1),
			},
			expected: 87.5,
		},
		{
			name: "Northeast facing, street exposure, floor 1",
			unit: &api.UnitMetadata{
				Facing:   ptr(api.NE),
				Exposure: ptr(api.Street),
				Floor:    ptr(1),
			},
			expected: 62.5,
		},
		{
			name: "Northwest facing, street exposure, floor 1",
			unit: &api.UnitMetadata{
				Facing:   ptr(api.NW),
				Exposure: ptr(api.Street),
				Floor:    ptr(1),
			},
			expected: 57.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoring.CalculateSolarScore(tt.unit)
			if score != tt.expected {
				t.Errorf("expected score %f, got %f", tt.expected, score)
			}
		})
	}
}

func TestCalculateSolarScore_ExposureAndFloor(t *testing.T) {
	tests := []struct {
		name     string
		unit     *api.UnitMetadata
		expected float32
	}{
		{
			name: "North facing courtyard on floor 1",
			unit: &api.UnitMetadata{
				Facing:   ptr(api.N),
				Exposure: ptr(api.Courtyard),
				Floor:    ptr(1),
			},
			// 40.0 * 0.65 = 26.0
			expected: 26.0,
		},
		{
			name: "North facing alley on floor 1",
			unit: &api.UnitMetadata{
				Facing:   ptr(api.N),
				Exposure: ptr(api.Alley),
				Floor:    ptr(1),
			},
			// 40.0 * 0.50 = 20.0
			expected: 20.0,
		},
		{
			name: "East facing street on floor 4 (floor bonus +10)",
			unit: &api.UnitMetadata{
				Facing:   ptr(api.E),
				Exposure: ptr(api.Street),
				Floor:    ptr(4),
			},
			// 85.0 + 10.0 = 95.0
			expected: 95.0,
		},
		{
			name: "Southeast facing courtyard on floor 3",
			unit: &api.UnitMetadata{
				Facing:   ptr(api.SE),
				Exposure: ptr(api.Courtyard),
				Floor:    ptr(3),
			},
			// 92.5 * 0.65 = 60.125 + 5.0 = 65.125 -> 65.1
			expected: 65.1,
		},
		{
			name: "South facing street on floor 10 (clamped at 100)",
			unit: &api.UnitMetadata{
				Facing:   ptr(api.S),
				Exposure: ptr(api.Street),
				Floor:    ptr(10),
			},
			// 100.0 + 40.0 = 140.0 -> clamped to 100.0
			expected: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoring.CalculateSolarScore(tt.unit)
			if score != tt.expected {
				t.Errorf("expected score %f, got %f", tt.expected, score)
			}
		})
	}
}

func TestSolarCalculator_Calculate(t *testing.T) {
	calc := scoring.NewSolarCalculator()
	if calc.Name() != scoring.FactorSolar {
		t.Errorf("expected name %q, got %q", scoring.FactorSolar, calc.Name())
	}

	res, err := calc.Calculate(context.Background(), scoring.ScoreParams{
		Unit: &api.UnitMetadata{
			Facing:   ptr(api.S),
			Exposure: ptr(api.Street),
			Floor:    ptr(1),
		},
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
