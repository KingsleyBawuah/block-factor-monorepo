package scoring

import (
	"context"
	"math"
	"strings"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
)

// SolarCalculator calculates the S_solar livability factor.
type SolarCalculator struct{}

// NewSolarCalculator creates a new SolarCalculator instance.
func NewSolarCalculator() *SolarCalculator {
	return &SolarCalculator{}
}

// Name returns FactorSolar.
func (c *SolarCalculator) Name() FactorName {
	return FactorSolar
}

// Calculate computes the S_solar score from unit metadata, automatically inferring
// the building's street-facing window direction if not explicitly supplied.
func (c *SolarCalculator) Calculate(_ context.Context, params ScoreParams) (FactorResult, error) {
	inferredFacing := api.S
	switch {
	case params.Address != nil:
		inferredFacing = InferStreetFacingOrientation(*params.Address)
	case params.AddressHint != nil:
		// A client that resolved `origin` itself (e.g. from its own prior
		// geocode call) can pass the matching address text here purely for
		// orientation inference, without that text ever being re-geocoded
		// -- see ScoreRequest.addressHint's doc comment (docs/openapi.yaml).
		inferredFacing = InferStreetFacingOrientation(*params.AddressHint)
	}

	score := CalculateSolarScore(params.Unit, inferredFacing)
	return FactorResult{
		Name:     FactorSolar,
		Score:    &score,
		Degraded: false,
	}, nil
}

// CalculateSolarScore computes the solar exposure sub-score based on PRD §5.8:
//   - Base orientation lookup:
//     South (100), SE (92.5), SW (87.5), East (85), West (75), NE (62.5), NW (57.5), North (40).
//   - Exposure modifier:
//     Street (1.0x), Courtyard (0.65x), Alley (0.50x).
//   - Floor bonus:
//     +5 pts per floor above floor 2 (e.g. floor 3 = +5, floor 5 = +15).
//   - Score formula:
//     clamp(BaseScore × ExposureModifier + FloorBonus, 0, 100).
func CalculateSolarScore(unit *api.UnitMetadata, defaultFacing api.UnitMetadataFacing) float32 {
	facing := defaultFacing
	if unit != nil && unit.Facing != nil {
		facing = *unit.Facing
	}

	baseScore := 100.0
	switch strings.ToUpper(string(facing)) {
	case "S":
		baseScore = 100.0
	case "SE":
		baseScore = 92.5
	case "SW":
		baseScore = 87.5
	case "E":
		baseScore = 85.0
	case "W":
		baseScore = 75.0
	case "NE":
		baseScore = 62.5
	case "NW":
		baseScore = 57.5
	case "N":
		baseScore = 40.0
	}

	exposureMod := 1.0 // Default when exposure is unspecified
	if unit != nil && unit.Exposure != nil {
		switch strings.ToLower(string(*unit.Exposure)) {
		case "street":
			exposureMod = 1.0
		case "courtyard":
			exposureMod = 0.65
		case "alley":
			exposureMod = 0.50
		}
	}

	floorBonus := 0.0
	if unit != nil && unit.Floor != nil && *unit.Floor > 2 {
		floorBonus = float64(*unit.Floor-2) * 5.0
	}

	rawScore := (baseScore * exposureMod) + floorBonus
	clamped := math.Min(100.0, math.Max(0.0, rawScore))
	return float32(math.Round(clamped*10) / 10)
}
