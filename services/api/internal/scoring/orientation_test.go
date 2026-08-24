package scoring_test

import (
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/scoring"
)

func TestInferStreetFacingOrientation_ChicagoGrid(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected api.UnitMetadataFacing
	}{
		{
			name:     "1801 W 18th St (Odd on East-West street -> faces North)",
			address:  "1801 W 18th St, Chicago, IL 60608",
			expected: api.N,
		},
		{
			name:     "1802 W 18th St (Even on East-West street -> faces South)",
			address:  "1802 W 18th St, Chicago, IL 60608",
			expected: api.S,
		},

		{
			name:     "2400 N Halsted St (Even on North-South street -> faces East)",
			address:  "2400 N Halsted St, Chicago, IL 60614",
			expected: api.E,
		},
		{
			name:     "2401 N Halsted St (Odd on North-South street -> faces West)",
			address:  "2401 N Halsted St, Chicago, IL 60614",
			expected: api.W,
		},
		{
			name:     "1600 N Milwaukee Ave (Even on NW-SE diagonal -> faces Southwest)",
			address:  "1600 N Milwaukee Ave, Chicago, IL 60647",
			expected: api.SW,
		},
		{
			name:     "1601 N Milwaukee Ave (Odd on NW-SE diagonal -> faces Northeast)",
			address:  "1601 N Milwaukee Ave, Chicago, IL 60647",
			expected: api.NE,
		},
		{
			name:     "2100 S Archer Ave (Even on SW-NE diagonal -> faces Southeast)",
			address:  "2100 S Archer Ave, Chicago, IL 60616",
			expected: api.SE,
		},
		{
			name:     "2101 S Archer Ave (Odd on SW-NE diagonal -> faces Northwest)",
			address:  "2101 S Archer Ave, Chicago, IL 60616",
			expected: api.NW,
		},
		{
			name:     "Empty address fallback to default South",
			address:  "",
			expected: api.S,
		},
		{
			name:     "5500 W Fakestreet Ave (unrecognized street name, even, W prefix -> faces South)",
			address:  "5500 W Fakestreet Ave, Chicago, IL 60651",
			expected: api.S,
		},
		{
			name:     "5501 W Fakestreet Ave (unrecognized street name, odd, W prefix -> faces North)",
			address:  "5501 W Fakestreet Ave, Chicago, IL 60651",
			expected: api.N,
		},
		{
			name:     "1500 N Fakestreet Ave (unrecognized street name, even, N prefix -> faces East)",
			address:  "1500 N Fakestreet Ave, Chicago, IL 60614",
			expected: api.E,
		},
		{
			name:     "1501 N Fakestreet Ave (unrecognized street name, odd, N prefix -> faces West)",
			address:  "1501 N Fakestreet Ave, Chicago, IL 60614",
			expected: api.W,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoring.InferStreetFacingOrientation(tt.address)
			if got != tt.expected {
				t.Errorf("InferStreetFacingOrientation(%q) = %q, expected %q", tt.address, got, tt.expected)
			}
		})
	}
}

func TestAzimuthToCompassFacing(t *testing.T) {
	tests := []struct {
		deg      float64
		expected api.UnitMetadataFacing
	}{
		{deg: 0, expected: api.N},
		{deg: 45, expected: api.NE},
		{deg: 90, expected: api.E},
		{deg: 135, expected: api.SE},
		{deg: 180, expected: api.S},
		{deg: 225, expected: api.SW},
		{deg: 270, expected: api.W},
		{deg: 315, expected: api.NW},
		{deg: 360, expected: api.N},
		{deg: -90, expected: api.W},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := scoring.AzimuthToCompassFacing(tt.deg)
			if got != tt.expected {
				t.Errorf("AzimuthToCompassFacing(%f) = %q, expected %q", tt.deg, got, tt.expected)
			}
		})
	}
}
