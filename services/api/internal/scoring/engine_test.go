package scoring_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
)

func ptr[T any](v T) *T {
	return &v
}

func TestGetDefaultWeights(t *testing.T) {
	remoteWeights := scoring.GetDefaultWeights(false)
	var remoteSum float64
	for _, w := range remoteWeights {
		remoteSum += w
	}
	if remoteSum != 100.0 {
		t.Errorf("expected remote weights sum to 100.0, got %f", remoteSum)
	}
	if remoteWeights[scoring.FactorCommute] != 0.0 {
		t.Errorf("expected commute weight to be 0 for remote preset, got %f", remoteWeights[scoring.FactorCommute])
	}

	commuterWeights := scoring.GetDefaultWeights(true)
	var commuterSum float64
	for _, w := range commuterWeights {
		commuterSum += w
	}
	if commuterSum != 100.0 {
		t.Errorf("expected commuter weights sum to 100.0, got %f", commuterSum)
	}
	if commuterWeights[scoring.FactorCommute] != 17.0 {
		t.Errorf("expected commute weight to be 17.0 for commuter preset, got %f", commuterWeights[scoring.FactorCommute])
	}
}

func TestResolveActiveWeights_WithOverrides(t *testing.T) {
	overrides := &api.WeightOverrides{
		Transit: ptr(float32(40.0)),
		Grocery: ptr(float32(30.0)),
	}

	weights := scoring.ResolveActiveWeights(overrides, false)
	if weights[scoring.FactorTransit] != 40.0 {
		t.Errorf("expected overridden transit weight 40.0, got %f", weights[scoring.FactorTransit])
	}
	if weights[scoring.FactorGrocery] != 30.0 {
		t.Errorf("expected overridden grocery weight 30.0, got %f", weights[scoring.FactorGrocery])
	}
	// Fallback to default for bike
	if weights[scoring.FactorBike] != 12.0 {
		t.Errorf("expected default bike weight 12.0, got %f", weights[scoring.FactorBike])
	}
}

func TestComputeCompositeScore_Calculation(t *testing.T) {
	results := map[scoring.FactorName]scoring.FactorResult{
		scoring.FactorTransit: {Name: scoring.FactorTransit, Score: ptr(float32(80.0))},
		scoring.FactorGrocery: {Name: scoring.FactorGrocery, Score: ptr(float32(60.0))},
	}
	weights := map[scoring.FactorName]float64{
		scoring.FactorTransit: 50.0,
		scoring.FactorGrocery: 50.0,
	}

	composite, err := scoring.ComputeCompositeScore(results, weights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// (80*50 + 60*50) / 100 = 70.0
	if composite != 70.0 {
		t.Errorf("expected composite score 70.0, got %f", composite)
	}
}

func TestComputeCompositeScore_GracefulDegradation(t *testing.T) {
	results := map[scoring.FactorName]scoring.FactorResult{
		scoring.FactorTransit: {Name: scoring.FactorTransit, Score: ptr(float32(90.0))},
		scoring.FactorGrocery: {Name: scoring.FactorGrocery, Score: nil, Degraded: true},
	}
	weights := map[scoring.FactorName]float64{
		scoring.FactorTransit: 25.0,
		scoring.FactorGrocery: 25.0,
	}

	composite, err := scoring.ComputeCompositeScore(results, weights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Grocery degraded -> only Transit active -> 90.0
	if composite != 90.0 {
		t.Errorf("expected renormalized composite score 90.0, got %f", composite)
	}
}

func TestComputeCompositeScore_AllZeroWeights(t *testing.T) {
	results := map[scoring.FactorName]scoring.FactorResult{
		scoring.FactorTransit: {Name: scoring.FactorTransit, Score: ptr(float32(90.0))},
	}
	weights := map[scoring.FactorName]float64{
		scoring.FactorTransit: 0.0,
	}

	_, err := scoring.ComputeCompositeScore(results, weights)
	if err == nil {
		t.Fatal("expected error when all weights sum to zero, got nil")
	}
}

type mockCalculator struct {
	name     scoring.FactorName
	calcFunc func(ctx context.Context, params scoring.ScoreParams) (scoring.FactorResult, error)
}

func (m *mockCalculator) Name() scoring.FactorName {
	return m.name
}

func (m *mockCalculator) Calculate(ctx context.Context, params scoring.ScoreParams) (scoring.FactorResult, error) {
	return m.calcFunc(ctx, params)
}

func TestEngine_Score_ConcurrentExecution(t *testing.T) {
	calcTransit := &mockCalculator{
		name: scoring.FactorTransit,
		calcFunc: func(_ context.Context, _ scoring.ScoreParams) (scoring.FactorResult, error) {
			return scoring.FactorResult{
				Name:  scoring.FactorTransit,
				Score: ptr(float32(100.0)),
			}, nil
		},
	}
	calcSolar := &mockCalculator{
		name: scoring.FactorSolar,
		calcFunc: func(_ context.Context, _ scoring.ScoreParams) (scoring.FactorResult, error) {
			return scoring.FactorResult{
				Name:  scoring.FactorSolar,
				Score: ptr(float32(80.0)),
			}, nil
		},
	}

	engine := scoring.NewEngine([]scoring.FactorCalculator{calcTransit, calcSolar}, slog.New(slog.DiscardHandler))

	params := scoring.ScoreParams{
		Origin: api.Coordinates{Lat: 41.8781, Lon: -87.6298},
		Weights: &api.WeightOverrides{
			Transit: ptr(float32(50.0)),
			Solar:   ptr(float32(50.0)),
		},
	}

	score, results, err := engine.Score(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if score != 90.0 {
		t.Errorf("expected score 90.0, got %f", score)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestEngine_Score_CalculatorFailureDegrades(t *testing.T) {
	calcTransit := &mockCalculator{
		name: scoring.FactorTransit,
		calcFunc: func(_ context.Context, _ scoring.ScoreParams) (scoring.FactorResult, error) {
			return scoring.FactorResult{
				Name:  scoring.FactorTransit,
				Score: ptr(float32(100.0)),
			}, nil
		},
	}
	calcFailing := &mockCalculator{
		name: scoring.FactorFlood,
		calcFunc: func(_ context.Context, _ scoring.ScoreParams) (scoring.FactorResult, error) {
			return scoring.FactorResult{}, errors.New("database timeout")
		},
	}

	engine := scoring.NewEngine([]scoring.FactorCalculator{calcTransit, calcFailing}, slog.New(slog.DiscardHandler))

	params := scoring.ScoreParams{
		Origin: api.Coordinates{Lat: 41.8781, Lon: -87.6298},
		Weights: &api.WeightOverrides{
			Transit: ptr(float32(50.0)),
			Flood:   ptr(float32(50.0)),
		},
	}

	score, results, err := engine.Score(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if score != 100.0 {
		t.Errorf("expected renormalized score 100.0, got %f", score)
	}
	if !results[scoring.FactorFlood].Degraded {
		t.Errorf("expected flood factor to be marked degraded")
	}
}
