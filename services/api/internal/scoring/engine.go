package scoring

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
)

const (
	defaultParentDeadline = 8 * time.Second
)

// GetDefaultWeights returns the default weight preset based on PRD §3.4.
func GetDefaultWeights(hasDestination bool) map[FactorName]float64 {
	if hasDestination {
		// Hybrid / In-Office Commuter Preset (sums to 100)
		return map[FactorName]float64{
			FactorTransit:    17.0,
			FactorCommute:    17.0,
			FactorBike:       10.0,
			FactorGrocery:    13.0,
			FactorThirdPlace: 9.0,
			FactorGreen:      9.0,
			FactorNoise:      10.0,
			FactorSafety:     10.0,
			FactorFlood:      3.0,
			FactorSolar:      2.0,
		}
	}

	// Remote Worker Preset (Default, commute hidden, sums to 100)
	return map[FactorName]float64{
		FactorTransit:    25.0,
		FactorCommute:    0.0,
		FactorBike:       12.0,
		FactorGrocery:    18.0,
		FactorThirdPlace: 13.0,
		FactorGreen:      9.0,
		FactorNoise:      9.0,
		FactorSafety:     9.0,
		FactorFlood:      3.0,
		FactorSolar:      2.0,
	}
}

// ResolveActiveWeights merges default presets with user-provided weight overrides.
func ResolveActiveWeights(overrides *api.WeightOverrides, hasDestination bool) map[FactorName]float64 {
	weights := GetDefaultWeights(hasDestination)
	if overrides == nil {
		return weights
	}

	if overrides.Transit != nil {
		weights[FactorTransit] = math.Max(0.0, float64(*overrides.Transit))
	}
	if overrides.Commute != nil && hasDestination {
		weights[FactorCommute] = math.Max(0.0, float64(*overrides.Commute))
	}
	if overrides.Bike != nil {
		weights[FactorBike] = math.Max(0.0, float64(*overrides.Bike))
	}
	if overrides.Grocery != nil {
		weights[FactorGrocery] = math.Max(0.0, float64(*overrides.Grocery))
	}
	if overrides.ThirdPlace != nil {
		weights[FactorThirdPlace] = math.Max(0.0, float64(*overrides.ThirdPlace))
	}
	if overrides.Green != nil {
		weights[FactorGreen] = math.Max(0.0, float64(*overrides.Green))
	}
	if overrides.Noise != nil {
		weights[FactorNoise] = math.Max(0.0, float64(*overrides.Noise))
	}
	if overrides.Safety != nil {
		weights[FactorSafety] = math.Max(0.0, float64(*overrides.Safety))
	}
	if overrides.Flood != nil {
		weights[FactorFlood] = math.Max(0.0, float64(*overrides.Flood))
	}
	if overrides.Solar != nil {
		weights[FactorSolar] = math.Max(0.0, float64(*overrides.Solar))
	}

	if !hasDestination {
		weights[FactorCommute] = 0.0
	}

	return weights
}

// ComputeCompositeScore calculates the weighted average composite score (0-100)
// per PRD §5.1: BlockFactor = Σ(w_i · S_i) / Σ(w_i).
// Factors that are degraded or lack data are excluded from the weight sum,
// and the remaining active factor weights automatically renormalize.
func ComputeCompositeScore(results map[FactorName]FactorResult, activeWeights map[FactorName]float64) (float32, error) {
	var sumWeightedScores float64
	var sumActiveWeights float64

	for factor, res := range results {
		if res.Degraded || res.Score == nil {
			continue
		}

		weight := activeWeights[factor]
		if weight <= 0 {
			continue
		}

		score := math.Min(100.0, math.Max(0.0, float64(*res.Score)))
		sumWeightedScores += weight * score
		sumActiveWeights += weight
	}

	if sumActiveWeights == 0 {
		return 0, errors.New("scoring: all active factor weights sum to zero")
	}

	composite := sumWeightedScores / sumActiveWeights
	composite = math.Min(100.0, math.Max(0.0, composite))
	return float32(math.Round(composite*10) / 10), nil
}

// Engine coordinates concurrent factor calculations and composite livability scoring.
type Engine struct {
	calculators map[FactorName]FactorCalculator
	logger      *slog.Logger
	timeout     time.Duration
}

// NewEngine constructs a new Engine with registered factor calculators.
func NewEngine(calculators []FactorCalculator, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}

	calcMap := make(map[FactorName]FactorCalculator, len(calculators))
	for _, c := range calculators {
		calcMap[c.Name()] = c
	}

	return &Engine{
		calculators: calcMap,
		logger:      logger,
		timeout:     defaultParentDeadline,
	}
}

// Score executes active factor calculators concurrently, computes the composite score,
// and assembles the score response factors.
func (e *Engine) Score(ctx context.Context, params ScoreParams) (float32, map[FactorName]FactorResult, error) {
	hasDestination := params.DestinationCoordinates != nil
	activeWeights := ResolveActiveWeights(params.Weights, hasDestination)

	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	results := make(map[FactorName]FactorResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, calc := range e.calculators {
		if name == FactorCommute && !hasDestination {
			continue
		}

		wg.Add(1)
		go func(n FactorName, c FactorCalculator) {
			defer wg.Done()

			res, err := c.Calculate(ctx, params)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				e.logger.Warn("factor calculation failed or timed out",
					slog.String("factor", string(n)),
					slog.String("error", err.Error()),
				)
				results[n] = FactorResult{
					Name:     n,
					Score:    nil,
					Degraded: true,
				}
				return
			}
			results[n] = res
		}(name, calc)
	}

	wg.Wait()

	compositeScore, err := ComputeCompositeScore(results, activeWeights)
	if err != nil {
		return 0, results, fmt.Errorf("scoring: compute composite score: %w", err)
	}

	return compositeScore, results, nil
}
