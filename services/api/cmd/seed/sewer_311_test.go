package main

import (
	"testing"
	"time"
)

func TestParseSewer311Row_Valid(t *testing.T) {
	tests := []struct {
		name         string
		row          sewer311Row
		expectedDate time.Time
		expectedLon  float64
		expectedLat  float64
	}{
		{
			name: "with fractional seconds",
			row: sewer311Row{
				SRNumber:    "SR24-01602354",
				SRType:      "Water in Basement Complaint",
				CreatedDate: "2024-08-23T09:05:36.000",
				Duplicate:   false,
				Latitude:    "41.668713",
				Longitude:   "-87.628077",
			},
			expectedDate: time.Date(2024, 8, 23, 9, 5, 36, 0, time.UTC),
			expectedLon:  -87.628077,
			expectedLat:  41.668713,
		},
		{
			name: "without fractional seconds",
			row: sewer311Row{
				SRNumber:    "SR25-00012345",
				SRType:      "Water in Basement Complaint",
				CreatedDate: "2025-01-15T14:30:00",
				Duplicate:   false,
				Latitude:    "41.8781",
				Longitude:   "-87.6298",
			},
			expectedDate: time.Date(2025, 1, 15, 14, 30, 0, 0, time.UTC),
			expectedLon:  -87.6298,
			expectedLat:  41.8781,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, ok, err := parseSewer311Row(tt.row)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !ok {
				t.Fatal("expected ok to be true")
			}
			if parsed.sourceID != tt.row.SRNumber {
				t.Errorf("got sourceID %q, want %q", parsed.sourceID, tt.row.SRNumber)
			}
			if !parsed.date.Equal(tt.expectedDate) {
				t.Errorf("got date %v, want %v", parsed.date, tt.expectedDate)
			}
			if parsed.lon != tt.expectedLon {
				t.Errorf("got lon %f, want %f", parsed.lon, tt.expectedLon)
			}
			if parsed.lat != tt.expectedLat {
				t.Errorf("got lat %f, want %f", parsed.lat, tt.expectedLat)
			}
		})
	}
}

func TestParseSewer311Row_MissingCoordinates(t *testing.T) {
	tests := []struct {
		name string
		row  sewer311Row
	}{
		{
			name: "empty latitude",
			row: sewer311Row{
				SRNumber:    "SR24-00000001",
				CreatedDate: "2024-08-23T09:05:36.000",
				Latitude:    "",
				Longitude:   "-87.628077",
			},
		},
		{
			name: "empty longitude",
			row: sewer311Row{
				SRNumber:    "SR24-00000002",
				CreatedDate: "2024-08-23T09:05:36.000",
				Latitude:    "41.668713",
				Longitude:   "",
			},
		},
		{
			name: "both empty",
			row: sewer311Row{
				SRNumber:    "SR24-00000003",
				CreatedDate: "2024-08-23T09:05:36.000",
				Latitude:    "",
				Longitude:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok, err := parseSewer311Row(tt.row)
			if err != nil {
				t.Fatalf("expected nil error for missing coords, got: %v", err)
			}
			if ok {
				t.Errorf("expected ok=false for missing coords")
			}
		})
	}
}

func TestParseSewer311Row_InvalidInputs(t *testing.T) {
	tests := []struct {
		name string
		row  sewer311Row
	}{
		{
			name: "empty sr_number",
			row: sewer311Row{
				SRNumber:    "",
				CreatedDate: "2024-08-23T09:05:36.000",
				Latitude:    "41.668713",
				Longitude:   "-87.628077",
			},
		},
		{
			name: "invalid latitude",
			row: sewer311Row{
				SRNumber:    "SR24-00000004",
				CreatedDate: "2024-08-23T09:05:36.000",
				Latitude:    "not-a-number",
				Longitude:   "-87.628077",
			},
		},
		{
			name: "invalid longitude",
			row: sewer311Row{
				SRNumber:    "SR24-00000005",
				CreatedDate: "2024-08-23T09:05:36.000",
				Latitude:    "41.668713",
				Longitude:   "not-a-number",
			},
		},
		{
			name: "invalid date layout",
			row: sewer311Row{
				SRNumber:    "SR24-00000006",
				CreatedDate: "invalid-date",
				Latitude:    "41.668713",
				Longitude:   "-87.628077",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseSewer311Row(tt.row)
			if err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}
