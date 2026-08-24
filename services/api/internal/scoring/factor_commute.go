package scoring

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

// Commute scoring bands, bonus, and penalty constants (PRD §5.10).
const (
	// CommuteExcellentThresholdMinutes/... are the door-to-door duration
	// cutoffs for each score band. Boundary values fall into the better
	// (higher-scoring) band, per PRD §5.1's half-open interval semantics.
	CommuteExcellentThresholdMinutes    = 20.0
	CommuteGoodThresholdMinutes         = 35.0
	CommuteManageableThresholdMinutes   = 50.0
	CommuteHighFrictionThresholdMinutes = 65.0

	CommuteScoreExcellent    = 100.0
	CommuteScoreGood         = 85.0
	CommuteScoreManageable   = 55.0
	CommuteScoreHighFriction = 25.0
	CommuteScoreSevere       = 0.0

	// CommuteOneSeatBonus rewards a direct (zero-transfer) itinerary.
	CommuteOneSeatBonus = 10.0

	// CommuteFreeTransfers is the number of transfers that incur no
	// penalty -- the PRD's friction penalty applies only "beyond 1 transfer".
	CommuteFreeTransfers = 1
	// CommuteTransferPenaltyPerTransfer is deducted for each transfer
	// beyond CommuteFreeTransfers.
	CommuteTransferPenaltyPerTransfer = 15.0

	// commuteDepartureHour/Minute is 8:30 AM -- PRD §5.10's simulated
	// rush-hour departure time.
	commuteDepartureHour   = 8
	commuteDepartureMinute = 30

	// commuteTimeZone is this app's only service area -- "Tuesday 8:30 AM"
	// means Chicago local time, not the server's own timezone.
	commuteTimeZone = "America/Chicago"
)

// CommuteQuerier plans a rush-hour transit itinerary between two
// coordinates. The concrete implementation (a live OTP2 GraphQL client)
// lands in Phase 6; Phase 3 tests this calculator against a fake.
type CommuteQuerier interface {
	// PlanTrip returns the best itinerary departing at or after departure,
	// or a nil itinerary (with a nil error) if no transit route exists --
	// distinct from an error, which means the query itself failed (e.g.
	// OTP2 unreachable, or its GTFS feed's service calendar has expired
	// for the requested date -- PRD §5.10).
	PlanTrip(ctx context.Context, origin, destination api.Coordinates, departure time.Time) (*clients.Itinerary, error)
}

// CommuteCalculator calculates the S_commute livability factor.
type CommuteCalculator struct {
	querier CommuteQuerier
	// now is overridable in tests so departure-time resolution doesn't
	// depend on the real wall clock.
	now func() time.Time
}

// NewCommuteCalculator creates a new CommuteCalculator.
func NewCommuteCalculator(querier CommuteQuerier) *CommuteCalculator {
	return &CommuteCalculator{querier: querier, now: time.Now}
}

// Name returns FactorCommute.
func (c *CommuteCalculator) Name() FactorName {
	return FactorCommute
}

// Calculate computes S_commute for the given origin/destination pair.
func (c *CommuteCalculator) Calculate(ctx context.Context, params ScoreParams) (FactorResult, error) {
	if c.querier == nil {
		return FactorResult{Name: FactorCommute, Degraded: true}, errors.New("scoring: commute querier not configured")
	}
	if params.DestinationCoordinates == nil {
		// The engine already skips calling Calculate for this factor when
		// no destination is set; this guard just makes the calculator safe
		// to call directly (e.g. in tests) without that skip logic.
		return FactorResult{Name: FactorCommute, Degraded: true}, nil
	}

	departure, err := nextTuesdayAt830AM(c.now(), commuteTimeZone)
	if err != nil {
		return FactorResult{Name: FactorCommute, Degraded: true}, err
	}

	itinerary, err := c.querier.PlanTrip(ctx, params.Origin, *params.DestinationCoordinates, departure)
	if err != nil {
		return FactorResult{Name: FactorCommute, Degraded: true}, err
	}
	if itinerary == nil {
		// No transit route exists for this pair -- there's nothing to
		// score, so degrade rather than report a misleading 0.
		return FactorResult{Name: FactorCommute, Degraded: true}, nil
	}

	score := CalculateCommuteScore(*itinerary)
	return FactorResult{
		Name:     FactorCommute,
		Score:    &score,
		Degraded: false,
	}, nil
}

// nextTuesdayAt830AM resolves PRD §5.10's simulated rush-hour departure:
// the next Tuesday 8:30 AM in the given IANA timezone, computed from now.
// If now itself falls on a Tuesday before 8:30 AM, that same day qualifies;
// otherwise it rolls to the following Tuesday. Whether that date also falls
// inside the live GTFS feed's service validity window is a live-data
// concern the Phase 6 querier implementation is responsible for -- this
// function only resolves the calendar date.
func nextTuesdayAt830AM(now time.Time, timeZone string) (time.Time, error) {
	loc, err := time.LoadLocation(timeZone)
	if err != nil {
		return time.Time{}, fmt.Errorf("scoring: load %s location: %w", timeZone, err)
	}

	local := now.In(loc)
	daysUntilTuesday := (int(time.Tuesday) - int(local.Weekday()) + 7) % 7
	candidate := time.Date(local.Year(), local.Month(), local.Day(), commuteDepartureHour, commuteDepartureMinute, 0, 0, loc).
		AddDate(0, 0, daysUntilTuesday)

	if daysUntilTuesday == 0 && local.After(candidate) {
		candidate = candidate.AddDate(0, 0, 7)
	}

	return candidate, nil
}

// CalculateCommuteScore computes the commute livability factor based on
// PRD §5.10:
//   - Door-to-door duration band: <= 20 min: 100, <= 35 min: 85,
//     <= 50 min: 55, <= 65 min: 25, otherwise: 0.
//   - One-Seat Ride Bonus: +10 pts if the itinerary has 0 transfers.
//   - Transfer Friction Penalty: -15 pts per transfer beyond 1 transfer.
//   - Formula: clamp(BaseScore + OneSeatBonus - TransferPenalty, 0, 100).
func CalculateCommuteScore(itinerary clients.Itinerary) float32 {
	durationMinutes := float64(itinerary.DurationSeconds) / 60.0

	var base float64
	switch {
	case durationMinutes <= CommuteExcellentThresholdMinutes:
		base = CommuteScoreExcellent
	case durationMinutes <= CommuteGoodThresholdMinutes:
		base = CommuteScoreGood
	case durationMinutes <= CommuteManageableThresholdMinutes:
		base = CommuteScoreManageable
	case durationMinutes <= CommuteHighFrictionThresholdMinutes:
		base = CommuteScoreHighFriction
	default:
		base = CommuteScoreSevere
	}

	oneSeatBonus := 0.0
	if itinerary.NumberOfTransfers == 0 {
		oneSeatBonus = CommuteOneSeatBonus
	}

	transferPenalty := 0.0
	if itinerary.NumberOfTransfers > CommuteFreeTransfers {
		transferPenalty = float64(itinerary.NumberOfTransfers-CommuteFreeTransfers) * CommuteTransferPenaltyPerTransfer
	}

	final := math.Min(100.0, math.Max(0.0, base+oneSeatBonus-transferPenalty))
	return float32(math.Round(final*10) / 10)
}
