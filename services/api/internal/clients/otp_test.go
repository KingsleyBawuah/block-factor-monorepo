package clients

import (
	"encoding/json"
	"testing"
)

// TestItinerary_DecodesRealisticGraphQLShapedResponse proves Itinerary/Leg/
// Route/TripPlace can decode a JSON payload shaped like the actual OTP2
// GTFS GraphQL API's planConnection `node` object (field names/nesting
// verified against OTP2's own schema.graphqls) -- the whole point of
// defining these types now, in Phase 3, is that the real HTTP client
// landing in Phase 6 is a drop-in rather than a rewrite.
func TestItinerary_DecodesRealisticGraphQLShapedResponse(t *testing.T) {
	// One planConnection edge's "node" field for a Loop-bound Pink Line
	// commute: walk to the station, ride direct, walk to the destination.
	const raw = `{
		"duration": 1080,
		"numberOfTransfers": 0,
		"legs": [
			{
				"mode": "WALK",
				"duration": 300,
				"route": null,
				"from": {"name": "Origin"},
				"to": {"name": "18th"}
			},
			{
				"mode": "RAIL",
				"duration": 600,
				"route": {"shortName": "Pink", "longName": "Pink Line"},
				"from": {"name": "18th"},
				"to": {"name": "Clark/Lake"}
			},
			{
				"mode": "WALK",
				"duration": 180,
				"route": null,
				"from": {"name": "Clark/Lake"},
				"to": {"name": "Destination"}
			}
		]
	}`

	var itinerary Itinerary
	if err := json.Unmarshal([]byte(raw), &itinerary); err != nil {
		t.Fatalf("unexpected error decoding fixture: %v", err)
	}

	if itinerary.DurationSeconds != 1080 {
		t.Errorf("expected duration 1080, got %d", itinerary.DurationSeconds)
	}
	if itinerary.NumberOfTransfers != 0 {
		t.Errorf("expected 0 transfers, got %d", itinerary.NumberOfTransfers)
	}
	if len(itinerary.Legs) != 3 {
		t.Fatalf("expected 3 legs, got %d", len(itinerary.Legs))
	}

	walkLeg := itinerary.Legs[0]
	if walkLeg.Mode != "WALK" || walkLeg.DurationSeconds != 300 || walkLeg.Route != nil {
		t.Errorf("unexpected walk leg: %+v", walkLeg)
	}
	if walkLeg.From.Name != "Origin" || walkLeg.To.Name != "18th" {
		t.Errorf("unexpected walk leg endpoints: %+v", walkLeg)
	}

	railLeg := itinerary.Legs[1]
	if railLeg.Mode != "RAIL" || railLeg.Route == nil {
		t.Fatalf("expected a non-nil route on the rail leg: %+v", railLeg)
	}
	if railLeg.Route.ShortName != "Pink" || railLeg.Route.LongName != "Pink Line" {
		t.Errorf("unexpected route: %+v", railLeg.Route)
	}

	finalWalkLeg := itinerary.Legs[2]
	if finalWalkLeg.Mode != "WALK" || finalWalkLeg.DurationSeconds != 180 || finalWalkLeg.Route != nil {
		t.Errorf("unexpected final walk leg: %+v", finalWalkLeg)
	}
	if finalWalkLeg.From.Name != "Clark/Lake" || finalWalkLeg.To.Name != "Destination" {
		t.Errorf("unexpected final walk leg endpoints: %+v", finalWalkLeg)
	}
}
