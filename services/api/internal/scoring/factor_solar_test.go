package scoring_test

import (
	"context"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
)

func TestCalculateSolarScore_NilUnit(t *testing.T) {
	score := scoring.CalculateSolarScore(nil, api.S)
	if score != 100.0 {
		t.Errorf("expected default score 100.0 for nil unit metadata facing South, got %f", score)
	}

	scoreNorth := scoring.CalculateSolarScore(nil, api.N)
	if scoreNorth != 40.0 {
		t.Errorf("expected default score 40.0 for nil unit metadata facing North, got %f", scoreNorth)
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
			score := scoring.CalculateSolarScore(tt.unit, api.S)
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
			score := scoring.CalculateSolarScore(tt.unit, api.S)
			if score != tt.expected {
				t.Errorf("expected score %f, got %f", tt.expected, score)
			}
		})
	}
}

func TestSolarCalculator_Calculate_AutoInferOrientation(t *testing.T) {
	calc := scoring.NewSolarCalculator()
	if calc.Name() != scoring.FactorSolar {
		t.Errorf("expected name %q, got %q", scoring.FactorSolar, calc.Name())
	}

	// 1801 W 18th St (Odd on East-West -> infers North -> base score 40.0)
	addr := "1801 W 18th St, Chicago, IL 60608"
	res, err := calc.Calculate(context.Background(), scoring.ScoreParams{
		Address: &addr,
		Unit: &api.UnitMetadata{
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
	if res.Score == nil || *res.Score != 40.0 {
		t.Errorf("expected inferred North score 40.0, got %v", res.Score)
	}

	// Explicit user override facing South should take precedence (score 100.0)
	resOverride, err := calc.Calculate(context.Background(), scoring.ScoreParams{
		Address: &addr,
		Unit: &api.UnitMetadata{
			Facing:   ptr(api.S),
			Exposure: ptr(api.Street),
			Floor:    ptr(1),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resOverride.Score == nil || *resOverride.Score != 100.0 {
		t.Errorf("expected override South score 100.0, got %v", resOverride.Score)
	}
}

func TestSolarCalculator_Calculate_AddressHint(t *testing.T) {
	calc := scoring.NewSolarCalculator()

	// AddressHint alone (no Address, as a request scoring by pre-resolved
	// Origin would send) must still drive real orientation inference rather
	// than silently defaulting to South -- this is the whole point of the
	// field (docs/openapi.yaml ScoreRequest.addressHint): avoid a second,
	// redundant geocode of the same address just to get an Address string.
	hint := "1801 W 18th St, Chicago, IL 60608" // odd, East-West -> North -> 40.0
	res, err := calc.Calculate(context.Background(), scoring.ScoreParams{
		AddressHint: &hint,
		Unit: &api.UnitMetadata{
			Exposure: ptr(api.Street),
			Floor:    ptr(1),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Score == nil || *res.Score != 40.0 {
		t.Errorf("expected AddressHint-inferred North score 40.0, got %v", res.Score)
	}

	// Address, when also present, takes precedence over AddressHint -- a
	// request should never send both, but the calculator's own priority
	// must be unambiguous rather than depend on which happens to be checked
	// last.
	addr := "1600 North Ave, Chicago, IL 60614"      // even, East-West -> South -> 100.0
	otherHint := "1801 W 18th St, Chicago, IL 60608" // would infer North -> 40.0 if used
	resBothSet, err := calc.Calculate(context.Background(), scoring.ScoreParams{
		Address:     &addr,
		AddressHint: &otherHint,
		Unit: &api.UnitMetadata{
			Exposure: ptr(api.Street),
			Floor:    ptr(1),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resBothSet.Score == nil || *resBothSet.Score != 100.0 {
		t.Errorf("expected Address to take precedence over AddressHint (South score 100.0), got %v", resBothSet.Score)
	}
}
