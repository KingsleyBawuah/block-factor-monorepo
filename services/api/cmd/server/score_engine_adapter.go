package main

import (
	"context"
	"errors"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
)

// scoreEngineAdapter satisfies api.ScoreEngine by delegating to a real
// *scoring.Engine, translating between api-owned types (the HTTP layer)
// and scoring-owned types (the factor calculators) at this one boundary --
// see api.ScoreEngine's doc comment for why those two packages can't
// import each other directly.
type scoreEngineAdapter struct {
	engine *scoring.Engine
}

func (a *scoreEngineAdapter) Score(ctx context.Context, params api.ScoreEngineParams) (float32, map[string]api.ScoreFactorResult, error) {
	composite, results, err := a.engine.Score(ctx, scoring.ScoreParams{
		Address:                params.Address,
		Origin:                 params.Origin,
		AddressHint:            params.AddressHint,
		DestinationCoordinates: params.DestinationCoordinates,
		Unit:                   params.Unit,
		Weights:                params.Weights,
	})
	if err != nil {
		if errors.Is(err, scoring.ErrInvalidWeights) {
			return 0, nil, api.ErrInvalidWeights
		}
		return 0, nil, err
	}

	factors := make(map[string]api.ScoreFactorResult, len(results))
	for name, res := range results {
		factors[string(name)] = api.ScoreFactorResult{
			Score:            res.Score,
			Degraded:         res.Degraded,
			DistanceMeters:   res.DistanceMeters,
			ContributingPois: res.ContributingPois,
		}
	}

	return composite, factors, nil
}
