package scoring

import (
	"context"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
)

// FactorName identifies a livability scoring factor.
type FactorName string

// Supported livability factor names.
const (
	FactorTransit    FactorName = "transit"
	FactorCommute    FactorName = "commute"
	FactorBike       FactorName = "bike"
	FactorGrocery    FactorName = "grocery"
	FactorThirdPlace FactorName = "thirdPlace"
	FactorGreen      FactorName = "green"
	FactorNoise      FactorName = "noise"
	FactorSafety     FactorName = "safety"
	FactorFlood      FactorName = "flood"
	FactorSolar      FactorName = "solar"
)

// FactorResult holds the outcome of a single factor calculation.
type FactorResult struct {
	Name             FactorName
	Score            *float32
	Degraded         bool
	DistanceMeters   *float32
	ContributingPois []api.ContributingPoi
}

// FactorCalculator is the interface implemented by each factor scoring module.
type FactorCalculator interface {
	Name() FactorName
	Calculate(ctx context.Context, params ScoreParams) (FactorResult, error)
}

// ScoreParams represents the input parameters provided to the scoring engine.
type ScoreParams struct {
	Address                *string
	Origin                 api.Coordinates
	AddressHint            *string
	DestinationCoordinates *api.Coordinates
	Unit                   *api.UnitMetadata
	Weights                *api.WeightOverrides
}
