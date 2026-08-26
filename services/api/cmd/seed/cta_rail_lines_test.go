package main

import (
	"reflect"
	"testing"
)

// TestParseRailLines exercises every distinct raw `lines` value confirmed
// live against the Chicago Data Portal (docs/DATA_SOURCES.md "CTA 'L'
// Lines") -- not a synthetic sample, so this locks in the exact real-world
// shapes the parser must handle: inconsistent "(Express)"/"(Exp)"
// abbreviation, Blue Line's two non-express branch qualifiers, and mixed
// single-line vs. multi-line-list rows.
func TestParseRailLines(t *testing.T) {
	tests := []struct {
		raw         string
		wantLines   []string
		wantExpress bool
	}{
		{"Blue Line (Forest Park)", []string{"Blue"}, false},
		{"Blue Line (O'Hare)", []string{"Blue"}, false},
		{"Brown Line", []string{"Brown"}, false},
		{"Brown, Green, Orange, Pink, Purple (Exp)", []string{"Brown", "Green", "Orange", "Pink", "Purple"}, true},
		{"Brown, Orange, Pink, Purple (Express)", []string{"Brown", "Orange", "Pink", "Purple"}, true},
		{"Brown, Purple", []string{"Brown", "Purple"}, false},
		{"Brown, Purple (Express), Red", []string{"Brown", "Purple", "Red"}, true},
		{"Green Line", []string{"Green"}, false},
		{"Green, Orange", []string{"Green", "Orange"}, false},
		{"Green, Pink", []string{"Green", "Pink"}, false},
		{"Orange Line", []string{"Orange"}, false},
		{"Pink Line", []string{"Pink"}, false},
		{"Purple Line", []string{"Purple"}, false},
		{"Red Line", []string{"Red"}, false},
		{"Red, Purple Line", []string{"Red", "Purple"}, false},
		{"Yellow Line", []string{"Yellow"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			gotLines, gotExpress := parseRailLines(tt.raw)
			if !reflect.DeepEqual(gotLines, tt.wantLines) {
				t.Errorf("parseRailLines(%q) lines = %v, want %v", tt.raw, gotLines, tt.wantLines)
			}
			if gotExpress != tt.wantExpress {
				t.Errorf("parseRailLines(%q) express = %v, want %v", tt.raw, gotExpress, tt.wantExpress)
			}
		})
	}
}

func TestRailGrade(t *testing.T) {
	tests := []struct {
		sourceType string
		want       string
		wantErr    bool
	}{
		{"Subway", "subway", false},
		{"Elevated or At Grade", "elevated_or_at_grade", false},
		{"Unknown", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.sourceType, func(t *testing.T) {
			got, err := railGrade(tt.sourceType)
			if (err != nil) != tt.wantErr {
				t.Fatalf("railGrade(%q) error = %v, wantErr %v", tt.sourceType, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("railGrade(%q) = %q, want %q", tt.sourceType, got, tt.want)
			}
		})
	}
}
