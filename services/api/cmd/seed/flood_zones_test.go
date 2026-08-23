package main

import "testing"

func TestKnownFloodZones(t *testing.T) {
	// Every value FEMA's published national standard defines must be
	// recognized (confirmed against FEMA's own glossary before the
	// migration's CHECK constraint was written).
	official := []string{"A", "AE", "AH", "AO", "AR", "A99", "V", "VE", "X", "D"}
	for _, zone := range official {
		if !knownFloodZones[zone] {
			t.Errorf("expected official zone %q to be recognized", zone)
		}
	}

	// OPEN WATER is a real value found in live Chicago data (marks literal
	// water bodies, not a risk-rated land parcel) but isn't part of the
	// official risk-zone standard -- must NOT be silently accepted.
	if knownFloodZones["OPEN WATER"] {
		t.Error("OPEN WATER should not be treated as a known risk zone designation")
	}
	if knownFloodZones["NOT A REAL ZONE"] {
		t.Error("an arbitrary unrecognized string should not be treated as known")
	}
}
