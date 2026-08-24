package clients

// Itinerary is a single trip plan result from OpenTripPlanner 2's GTFS
// GraphQL API (the `planConnection` query, under `data.planConnection.edges[].node`)
// -- OTP 2.x's only routing API surface; the legacy REST plan endpoint was
// removed. Field names/shapes mirror OTP2's own schema.graphqls
// Itinerary/Leg types directly (verified against the live OTP2 docs), not
// invented, and carry JSON tags matching the real GraphQL field names, so
// they can already decode a realistic response body.
//
// This is Phase 3 scope: types only, no HTTP client. factor_commute.go is
// built and unit-tested against these types via fixture-based fakes; the
// live GraphQL-calling client lands in Phase 6 (see IMPLEMENTATION_PHASES.md).
type Itinerary struct {
	// DurationSeconds is the GraphQL schema's `duration` field (Long,
	// seconds) -- door-to-door including walking, waiting, and transfers.
	DurationSeconds int64 `json:"duration"`
	// NumberOfTransfers is the GraphQL schema's `numberOfTransfers` field.
	NumberOfTransfers int   `json:"numberOfTransfers"`
	Legs              []Leg `json:"legs"`
}

// Leg is one leg of an Itinerary.
type Leg struct {
	// Mode is a GTFS GraphQL Mode enum value, e.g. "WALK", "BUS", "RAIL",
	// "SUBWAY", "TRAM".
	Mode string `json:"mode"`
	// DurationSeconds is the GraphQL schema's `duration` field (Float, seconds).
	DurationSeconds float64 `json:"duration"`
	// Route is nil for a WALK leg.
	Route *Route    `json:"route"`
	From  TripPlace `json:"from"`
	To    TripPlace `json:"to"`
}

// Route is the transit route a Leg rides, e.g. the CTA Pink Line.
type Route struct {
	ShortName string `json:"shortName"`
	LongName  string `json:"longName"`
}

// TripPlace is a Leg's endpoint -- named to avoid colliding with this
// package's unrelated Place type (a Google Places API result). Only the
// field BlockFactor currently needs (a human-readable name for itinerary
// summaries) is modeled; OTP2's real GraphQL Place type carries far more
// (lat/lon, scheduled/estimated times, stop metadata) that the Phase 6 live
// client can add as needed.
type TripPlace struct {
	Name string `json:"name"`
}
