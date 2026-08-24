package main

import (
	"context"
	"log/slog"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
)

type stubCalculator struct {
	name   scoring.FactorName
	result scoring.FactorResult
	err    error
}

func (c *stubCalculator) Name() scoring.FactorName { return c.name }

func (c *stubCalculator) Calculate(_ context.Context, _ scoring.ScoreParams) (scoring.FactorResult, error) {
	return c.result, c.err
}

func TestScoreEngineAdapter_Score_TranslatesResultsAndKeys(t *testing.T) {
	transitScore := float32(90.0)
	dist := float32(200.0)
	engine := scoring.NewEngine([]scoring.FactorCalculator{
		&stubCalculator{
			name: scoring.FactorTransit,
			result: scoring.FactorResult{
				Name:           scoring.FactorTransit,
				Score:          &transitScore,
				DistanceMeters: &dist,
				ContributingPois: []api.ContributingPoi{
					{Name: "Clark/Lake", DistanceMeters: 200.0},
				},
			},
		},
	}, slog.New(slog.DiscardHandler))

	adapter := &scoreEngineAdapter{engine: engine}

	composite, factors, err := adapter.Score(context.Background(), api.ScoreEngineParams{
		Origin: api.Coordinates{Lat: 41.8781, Lon: -87.6298},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if composite != 90.0 {
		t.Errorf("expected composite score 90.0 (single active factor), got %v", composite)
	}

	transit, ok := factors["transit"]
	if !ok {
		t.Fatalf("expected key %q in translated factors map, got keys %v", "transit", factorKeys(factors))
	}
	if transit.Score == nil || *transit.Score != 90.0 {
		t.Errorf("expected transit score 90.0, got %v", transit.Score)
	}
	if transit.DistanceMeters == nil || *transit.DistanceMeters != 200.0 {
		t.Errorf("expected distance 200.0, got %v", transit.DistanceMeters)
	}
	if len(transit.ContributingPois) != 1 || transit.ContributingPois[0].Name != "Clark/Lake" {
		t.Errorf("unexpected contributing pois: %+v", transit.ContributingPois)
	}
}

func TestScoreEngineAdapter_Score_PassesParamsThrough(t *testing.T) {
	var capturedParams scoring.ScoreParams
	engine := scoring.NewEngine([]scoring.FactorCalculator{
		&stubCalculatorCapturingParams{captured: &capturedParams},
	}, slog.New(slog.DiscardHandler))
	adapter := &scoreEngineAdapter{engine: engine}

	dest := api.Coordinates{Lat: 41.8858, Lon: -87.6353}
	weights := &api.WeightOverrides{Transit: floatPtr(20)}
	_, _, err := adapter.Score(context.Background(), api.ScoreEngineParams{
		Origin:                 api.Coordinates{Lat: 41.8781, Lon: -87.6298},
		DestinationCoordinates: &dest,
		Weights:                weights,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedParams.Origin.Lat != 41.8781 || capturedParams.Origin.Lon != -87.6298 {
		t.Errorf("unexpected origin: %+v", capturedParams.Origin)
	}
	if capturedParams.DestinationCoordinates == nil || *capturedParams.DestinationCoordinates != dest {
		t.Errorf("unexpected destination: %+v", capturedParams.DestinationCoordinates)
	}
	if capturedParams.Weights != weights {
		t.Errorf("expected weights passed through unchanged")
	}
}

func TestScoreEngineAdapter_Score_PropagatesEngineError(t *testing.T) {
	// An engine with only a zero-weight active factor forces
	// ComputeCompositeScore's "all active weights sum to zero" error.
	engine := scoring.NewEngine(nil, slog.New(slog.DiscardHandler))
	adapter := &scoreEngineAdapter{engine: engine}

	_, _, err := adapter.Score(context.Background(), api.ScoreEngineParams{
		Origin: api.Coordinates{Lat: 41.8781, Lon: -87.6298},
	})
	if err == nil {
		t.Fatal("expected an error when no calculators are registered, got nil")
	}
}

type stubCalculatorCapturingParams struct {
	captured *scoring.ScoreParams
}

func (c *stubCalculatorCapturingParams) Name() scoring.FactorName { return scoring.FactorTransit }

func (c *stubCalculatorCapturingParams) Calculate(_ context.Context, params scoring.ScoreParams) (scoring.FactorResult, error) {
	*c.captured = params
	score := float32(50.0)
	return scoring.FactorResult{Name: scoring.FactorTransit, Score: &score}, nil
}

func factorKeys(m map[string]api.ScoreFactorResult) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func floatPtr(v float32) *float32 { return &v }
