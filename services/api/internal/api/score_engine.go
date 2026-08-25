package api

import (
	"context"
	"errors"
)

// ErrInvalidWeights is the api-owned sentinel a ScoreEngine implementation
// should wrap/return when the request's resolved active weights sum to
// zero before any factor was even computed -- a genuine client input
// problem (e.g. every weight explicitly overridden to 0), mapped by
// ScoreAddress to 400 VALIDATION_ERROR. This mirrors
// internal/scoring.ErrInvalidWeights but is declared separately for the
// same import-cycle reason as ScoreEngineParams/ScoreFactorResult; the
// cmd/server adapter translates one into the other.
var ErrInvalidWeights = errors.New("api: active weights sum to zero")

// ScoreEngineParams are the inputs to a composite score computation.
//
// This mirrors internal/scoring.ScoreParams field-for-field, but is
// declared here (in api-owned types only) rather than reused directly,
// because of the package dependency direction: internal/scoring already
// imports internal/api for the Coordinates/WeightOverrides/UnitMetadata/
// ContributingPoi types its own ScoreParams and FactorResult are built
// from, so internal/api importing internal/scoring back would be an
// import cycle. cmd/server wires a thin adapter that satisfies ScoreEngine
// by calling the real *scoring.Engine and translating between the two
// packages' types at that one boundary.
type ScoreEngineParams struct {
	Address                *string
	Origin                 Coordinates
	DestinationCoordinates *Coordinates
	Unit                   *UnitMetadata
	Weights                *WeightOverrides
}

// ScoreFactorResult mirrors internal/scoring.FactorResult using only
// api-owned types -- see ScoreEngineParams for why.
type ScoreFactorResult struct {
	Score            *float32
	Degraded         bool
	DistanceMeters   *float32
	ContributingPois []ContributingPoi
}

// ScoreEngine computes per-factor sub-scores and a weighted composite
// score. factors is keyed by the same lowercase-camelCase factor names
// used in ScoreResponse.Factors' JSON ("transit", "commute", "bike", ...);
// a factor absent from the map (as opposed to present with Degraded:
// true) means it was never computed at all -- currently only "commute"
// when no destination was supplied.
type ScoreEngine interface {
	Score(ctx context.Context, params ScoreEngineParams) (compositeScore float32, factors map[string]ScoreFactorResult, err error)
}
