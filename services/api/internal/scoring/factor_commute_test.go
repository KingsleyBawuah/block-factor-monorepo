package scoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("failed to load location %q: %v", name, err)
	}
	return loc
}

func TestNextTuesdayAt830AM(t *testing.T) {
	chicago := mustLoadLocation(t, "America/Chicago")

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "Monday rolls to the next day (Tuesday)",
			now:  time.Date(2026, 8, 24, 9, 0, 0, 0, chicago), // a Monday
			want: time.Date(2026, 8, 25, 8, 30, 0, 0, chicago),
		},
		{
			name: "Tuesday before 8:30 AM stays same day",
			now:  time.Date(2026, 8, 25, 6, 0, 0, 0, chicago), // a Tuesday
			want: time.Date(2026, 8, 25, 8, 30, 0, 0, chicago),
		},
		{
			name: "Tuesday exactly at 8:30 AM stays same day",
			now:  time.Date(2026, 8, 25, 8, 30, 0, 0, chicago),
			want: time.Date(2026, 8, 25, 8, 30, 0, 0, chicago),
		},
		{
			name: "Tuesday just after 8:30 AM rolls to next week",
			now:  time.Date(2026, 8, 25, 8, 31, 0, 0, chicago),
			want: time.Date(2026, 9, 1, 8, 30, 0, 0, chicago),
		},
		{
			name: "Sunday rolls forward two days",
			now:  time.Date(2026, 8, 23, 12, 0, 0, 0, chicago), // a Sunday
			want: time.Date(2026, 8, 25, 8, 30, 0, 0, chicago),
		},
		{
			name: "input in a different timezone is converted before resolving",
			now:  time.Date(2026, 8, 25, 3, 0, 0, 0, time.UTC), // 22:00 Monday in Chicago (UTC-5 in August)
			want: time.Date(2026, 8, 25, 8, 30, 0, 0, chicago),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nextTuesdayAt830AM(tt.now, "America/Chicago")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
			if got.Weekday() != time.Tuesday {
				t.Errorf("expected a Tuesday, got %v", got.Weekday())
			}
		})
	}
}

func TestNextTuesdayAt830AM_InvalidTimeZone(t *testing.T) {
	_, err := nextTuesdayAt830AM(time.Now(), "Not/A_Real_Zone")
	if err == nil {
		t.Fatal("expected error for invalid timezone, got nil")
	}
}

func itinerary(durationMinutes float64, transfers int) clients.Itinerary {
	return clients.Itinerary{
		DurationSeconds:   int64(durationMinutes * 60),
		NumberOfTransfers: transfers,
	}
}

func TestCalculateCommuteScore_Scenarios(t *testing.T) {
	tests := []struct {
		name      string
		itinerary clients.Itinerary
		want      float32
	}{
		{"18 min, 0 transfers -> 100 base, +10 bonus clamped to 100", itinerary(18, 0), 100.0},
		{"exactly 20 min boundary -> excellent band (100)", itinerary(CommuteExcellentThresholdMinutes, 1), 100.0},
		{"just over 20 min -> good band (85)", itinerary(CommuteExcellentThresholdMinutes+0.1, 1), 85.0},
		{"exactly 35 min boundary -> good band (85)", itinerary(CommuteGoodThresholdMinutes, 1), 85.0},
		{"just over 35 min -> manageable band (55)", itinerary(CommuteGoodThresholdMinutes+0.1, 1), 55.0},
		{"exactly 50 min boundary -> manageable band (55)", itinerary(CommuteManageableThresholdMinutes, 1), 55.0},
		{"just over 50 min -> high-friction band (25)", itinerary(CommuteManageableThresholdMinutes+0.1, 1), 25.0},
		{"exactly 65 min boundary -> high-friction band (25)", itinerary(CommuteHighFrictionThresholdMinutes, 1), 25.0},
		{"just over 65 min -> severe (0)", itinerary(CommuteHighFrictionThresholdMinutes+0.1, 1), 0.0},
		{"30 min, 0 transfers: 85 base + 10 one-seat bonus = 95", itinerary(30, 0), 95.0},
		{"30 min, 1 transfer: 85 base, no bonus, no penalty (first transfer is free)", itinerary(30, 1), 85.0},
		{"30 min, 2 transfers: 85 base - 15 penalty = 70", itinerary(30, 2), 70.0},
		{"30 min, 3 transfers: 85 base - 30 penalty = 55", itinerary(30, 3), 55.0},
		{"severe duration with heavy transfers clamps at 0, not negative", itinerary(90, 5), 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateCommuteScore(tt.itinerary); got != tt.want {
				t.Errorf("expected score %v, got %v", tt.want, got)
			}
		})
	}
}

type mockCommuteQuerier struct {
	itinerary         *clients.Itinerary
	err               error
	capturedDeparture time.Time
}

func (m *mockCommuteQuerier) PlanTrip(_ context.Context, _, _ api.Coordinates, departure time.Time) (*clients.Itinerary, error) {
	m.capturedDeparture = departure
	if m.err != nil {
		return nil, m.err
	}
	return m.itinerary, nil
}

func TestCommuteCalculator_Calculate_Success(t *testing.T) {
	i := itinerary(18, 0)
	querier := &mockCommuteQuerier{itinerary: &i}

	calc := NewCommuteCalculator(querier)
	if calc.Name() != FactorCommute {
		t.Errorf("expected name %q, got %q", FactorCommute, calc.Name())
	}

	dest := api.Coordinates{Lat: 41.8858, Lon: -87.6353}
	res, err := calc.Calculate(context.Background(), ScoreParams{
		Origin:                 api.Coordinates{Lat: 41.8781, Lon: -87.6298},
		DestinationCoordinates: &dest,
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
	if querier.capturedDeparture.Weekday() != time.Tuesday {
		t.Errorf("expected departure to be a Tuesday, got %v", querier.capturedDeparture.Weekday())
	}
}

func TestCommuteCalculator_Calculate_NoDestinationDegradesWithoutError(t *testing.T) {
	querier := &mockCommuteQuerier{}
	calc := NewCommuteCalculator(querier)

	res, err := calc.Calculate(context.Background(), ScoreParams{
		Origin: api.Coordinates{Lat: 41.8781, Lon: -87.6298},
	})
	if err != nil {
		t.Fatalf("expected no destination to degrade gracefully, not error: %v", err)
	}
	if !res.Degraded {
		t.Errorf("expected degraded=true with no destination set")
	}
}

func TestCommuteCalculator_Calculate_NoItineraryFoundDegradesWithoutError(t *testing.T) {
	querier := &mockCommuteQuerier{itinerary: nil}
	calc := NewCommuteCalculator(querier)

	dest := api.Coordinates{Lat: 41.8858, Lon: -87.6353}
	res, err := calc.Calculate(context.Background(), ScoreParams{
		Origin:                 api.Coordinates{Lat: 41.8781, Lon: -87.6298},
		DestinationCoordinates: &dest,
	})
	if err != nil {
		t.Fatalf("expected a nil itinerary to degrade gracefully, not error: %v", err)
	}
	if !res.Degraded {
		t.Errorf("expected degraded=true when no itinerary is found")
	}
}

func TestCommuteCalculator_Calculate_QuerierErrorDegrades(t *testing.T) {
	querier := &mockCommuteQuerier{err: errors.New("OTP2 unreachable")}
	calc := NewCommuteCalculator(querier)

	dest := api.Coordinates{Lat: 41.8858, Lon: -87.6353}
	res, err := calc.Calculate(context.Background(), ScoreParams{
		Origin:                 api.Coordinates{Lat: 41.8781, Lon: -87.6298},
		DestinationCoordinates: &dest,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !res.Degraded {
		t.Errorf("expected degraded=true on querier error")
	}
}

func TestCommuteCalculator_Calculate_NotConfigured(t *testing.T) {
	calc := NewCommuteCalculator(nil)
	res, err := calc.Calculate(context.Background(), ScoreParams{
		Origin: api.Coordinates{Lat: 41.8781, Lon: -87.6298},
	})
	if err == nil {
		t.Fatal("expected error for unconfigured calculator, got nil")
	}
	if !res.Degraded {
		t.Errorf("expected degraded=true when unconfigured")
	}
}

func TestCommuteCalculator_Calculate_UsesInjectedClock(t *testing.T) {
	i := itinerary(18, 0)
	querier := &mockCommuteQuerier{itinerary: &i}
	calc := NewCommuteCalculator(querier)

	// A Wednesday -- the resolved departure must roll forward to the
	// following Tuesday, proving Calculate actually uses c.now() rather
	// than some other fixed/real-clock source.
	fixed := time.Date(2026, 8, 26, 12, 0, 0, 0, mustLoadLocation(t, "America/Chicago"))
	calc.now = func() time.Time { return fixed }

	dest := api.Coordinates{Lat: 41.8858, Lon: -87.6353}
	_, err := calc.Calculate(context.Background(), ScoreParams{
		Origin:                 api.Coordinates{Lat: 41.8781, Lon: -87.6298},
		DestinationCoordinates: &dest,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := time.Date(2026, 9, 1, 8, 30, 0, 0, mustLoadLocation(t, "America/Chicago"))
	if !querier.capturedDeparture.Equal(want) {
		t.Errorf("expected departure %v, got %v", want, querier.capturedDeparture)
	}
}
