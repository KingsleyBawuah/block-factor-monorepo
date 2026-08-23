package main

import (
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

func TestClassifyNoiseSource(t *testing.T) {
	tests := []struct {
		name      string
		tags      map[string]string
		wantType  string
		wantGrade string
		wantOK    bool
	}{
		{
			name:      "CTA track tagged subway but actually elevated (South Side Elevated)",
			tags:      map[string]string{"railway": "subway", "bridge": "yes", "layer": "2", "name": "South Side Elevated"},
			wantType:  "cta_track",
			wantGrade: "elevated",
			wantOK:    true,
		},
		{
			name:      "CTA track genuinely underground",
			tags:      map[string]string{"railway": "subway", "tunnel": "yes", "layer": "-2"},
			wantType:  "cta_track",
			wantGrade: "underground",
			wantOK:    true,
		},
		{
			name:      "CTA track at grade (no tunnel or bridge)",
			tags:      map[string]string{"railway": "subway", "name": "Lake Branch"},
			wantType:  "cta_track",
			wantGrade: "at_grade",
			wantOK:    true,
		},
		{
			name:     "expressway",
			tags:     map[string]string{"highway": "motorway", "name": "Kennedy Expressway"},
			wantType: "expressway",
			wantOK:   true,
		},
		{
			name:     "arterial",
			tags:     map[string]string{"highway": "primary", "name": "North Cicero Avenue"},
			wantType: "arterial",
			wantOK:   true,
		},
		{
			name: "unrelated road class",
			tags: map[string]string{"highway": "residential"},
		},
		{
			name: "unrelated railway",
			tags: map[string]string{"railway": "light_rail"},
		},
		{
			name: "no relevant tags",
			tags: map[string]string{"name": "Some Place"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotGrade, ok := classifyNoiseSource(tt.tags)
			if ok != tt.wantOK {
				t.Fatalf("classifyNoiseSource(%v) ok = %v, want %v", tt.tags, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if gotType != tt.wantType {
				t.Errorf("classifyNoiseSource(%v) type = %q, want %q", tt.tags, gotType, tt.wantType)
			}
			if gotGrade != tt.wantGrade {
				t.Errorf("classifyNoiseSource(%v) grade = %q, want %q", tt.tags, gotGrade, tt.wantGrade)
			}
		})
	}
}

func TestClassifyTrackGrade_LayerFallback(t *testing.T) {
	tests := []struct {
		name string
		tags map[string]string
		want string
	}{
		{"tunnel wins over layer", map[string]string{"tunnel": "yes", "layer": "1"}, "underground"},
		{"bridge wins over layer", map[string]string{"bridge": "yes", "layer": "-1"}, "elevated"},
		{"positive layer only, no tunnel/bridge", map[string]string{"layer": "1"}, "elevated"},
		{"negative layer only, no tunnel/bridge", map[string]string{"layer": "-1"}, "underground"},
		{"zero layer, no tunnel/bridge", map[string]string{"layer": "0"}, "at_grade"},
		{"no layer, no tunnel/bridge", map[string]string{}, "at_grade"},
		{"unparseable layer", map[string]string{"layer": "not-a-number"}, "at_grade"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyTrackGrade(tt.tags); got != tt.want {
				t.Errorf("classifyTrackGrade(%v) = %q, want %q", tt.tags, got, tt.want)
			}
		})
	}
}

func TestParseLanes(t *testing.T) {
	if got := parseLanes(map[string]string{"lanes": "4"}); got == nil || *got != 4 {
		t.Errorf("parseLanes = %v, want 4", got)
	}
	if got := parseLanes(map[string]string{}); got != nil {
		t.Errorf("parseLanes(missing) = %v, want nil", got)
	}
	if got := parseLanes(map[string]string{"lanes": "many"}); got != nil {
		t.Errorf("parseLanes(unparseable) = %v, want nil", got)
	}
}

func TestParseMaxspeedMPH(t *testing.T) {
	if got := parseMaxspeedMPH(map[string]string{"maxspeed": "45 mph"}); got == nil || *got != 45 {
		t.Errorf("parseMaxspeedMPH = %v, want 45", got)
	}
	if got := parseMaxspeedMPH(map[string]string{"maxspeed": "50 km/h"}); got != nil {
		t.Errorf("parseMaxspeedMPH(non-mph) = %v, want nil (unit not verified for this format)", got)
	}
	if got := parseMaxspeedMPH(map[string]string{}); got != nil {
		t.Errorf("parseMaxspeedMPH(missing) = %v, want nil", got)
	}
}

func TestHasTurnLane(t *testing.T) {
	tests := []struct {
		name string
		tags map[string]string
		want bool
	}{
		{"turn:lanes with left", map[string]string{"turn:lanes": "left|through|through"}, true},
		{"turn:lanes:forward with left", map[string]string{"turn:lanes:forward": "left|through"}, true},
		{"turn:lanes without left", map[string]string{"turn:lanes": "through|through;slight_right"}, false},
		{"no turn:lanes tags", map[string]string{"highway": "primary"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasTurnLane(tt.tags); got != tt.want {
				t.Errorf("hasTurnLane(%v) = %v, want %v", tt.tags, got, tt.want)
			}
		})
	}
}

func TestWayGeometryToWKT(t *testing.T) {
	geom := []clients.OverpassLatLon{
		{Lat: 41.85, Lon: -87.65},
		{Lat: 41.86, Lon: -87.66},
	}
	want := "LINESTRING(-87.650000 41.850000, -87.660000 41.860000)"
	if got := wayGeometryToWKT(geom); got != want {
		t.Errorf("wayGeometryToWKT() = %q, want %q", got, want)
	}
}
