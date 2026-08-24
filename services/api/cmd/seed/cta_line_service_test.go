package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCanonicalLineName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Red Line", "Red"},
		{"Purple Line", "Purple"},
		{"Yellow Line", "Yellow"},
		{"Pink Line", "Pink"},
		{" Blue Line ", "Blue"},
	}
	for _, tc := range tests {
		if got := canonicalLineName(tc.in); got != tc.want {
			t.Errorf("canonicalLineName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsInOvernightWindow(t *testing.T) {
	tests := []struct {
		secs int
		want bool
	}{
		{0, false},      // 00:00:00 (midnight)
		{3599, false},   // 00:59:59
		{3600, true},    // 01:00:00
		{7200, true},    // 02:00:00
		{14399, true},   // 03:59:59
		{14400, false},  // 04:00:00
		{89999, false},  // 24:59:59 (GTFS after-midnight, just before the window)
		{90000, true},   // 25:00:00 (GTFS after-midnight, window start)
		{100799, true},  // 27:59:59
		{100800, false}, // 28:00:00
	}
	for _, tc := range tests {
		if got := isInOvernightWindow(tc.secs); got != tc.want {
			t.Errorf("isInOvernightWindow(%d) = %v, want %v", tc.secs, got, tc.want)
		}
	}
}

// buildRailGTFSZip generates a synthetic GTFS archive modeling two rail lines:
// R_247 has both AM-peak and overnight service (24-hour-style line); R_DAY only
// has AM-peak service (no overnight trips at all, like most CTA lines).
func buildRailGTFSZip(t *testing.T) []byte {
	t.Helper()

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// Single always-valid weekday+weekend service window so referenceDate choice
	// inside 2026 doesn't matter for this fixture (the validity-window fix itself
	// is covered separately by bus_corridors_test.go).
	calendarCSV := `service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date
SER_WD,1,1,1,1,1,0,0,20260101,20261231
SER_SA,0,0,0,0,0,1,0,20260101,20261231
SER_SU,0,0,0,0,0,0,1,20260101,20261231
`
	routesCSV := `route_id,route_short_name,route_long_name,route_type
Red,,Red Line,1
Y,,Yellow Line,1
99,99,Not Rail,3
`

	var tripsBuilder strings.Builder
	tripsBuilder.WriteString("trip_id,route_id,service_id,direction_id,shape_id\n")
	var stopTimesBuilder strings.Builder
	stopTimesBuilder.WriteString("trip_id,arrival_time,departure_time,stop_id,stop_sequence\n")

	// Red Line: 6 AM-peak trips/direction (120/6 = 20.0 min AM headway) on SER_WD.
	for dir := 0; dir < 2; dir++ {
		for i := 0; i < 6; i++ {
			tID := fmt.Sprintf("TRIP_RED_AM_D%d_%d", dir, i)
			mins := i * 20
			h := 7 + mins/60
			m := mins % 60
			fmt.Fprintf(&tripsBuilder, "%s,Red,SER_WD,%d,SHAPE_RED\n", tID, dir)
			fmt.Fprintf(&stopTimesBuilder, "%s,%02d:%02d:00,%02d:%02d:00,STOP_R%d,1\n", tID, h, m, h, m, dir)
		}
	}
	// Red Line overnight service: deliberately UNEVEN across day-types, to prove
	// the headway is a true day-weighted weekly average and not an accidental
	// match from a uniform case. Per direction: 5 trips under SER_WD (5
	// nights/week), 2 under SER_SA (1 night/week), 1 under SER_SU (1 night/week).
	// Weighted trips/week = 5*5 + 2*1 + 1*1 = 28 -> 28/7 = 4.0 trips/night ->
	// 180/4.0 = 45.0 min mean overnight headway. A naive unweighted pool (the
	// pre-fix behavior) would instead sum 5+2+1=8 raw trips and compute
	// 180/8 = 22.5 min -- roughly 2x too frequent, exactly the class of bug
	// this fixture guards against (ARCHITECTURE.md Decision Log row 28).
	for dir := 0; dir < 2; dir++ {
		for i := 0; i < 5; i++ {
			tID := fmt.Sprintf("TRIP_RED_ON_WD_D%d_%d", dir, i)
			mins := i * 15
			h := 1 + mins/60
			m := mins % 60
			fmt.Fprintf(&tripsBuilder, "%s,Red,SER_WD,%d,SHAPE_RED\n", tID, dir)
			fmt.Fprintf(&stopTimesBuilder, "%s,%02d:%02d:00,%02d:%02d:00,STOP_R%d,1\n", tID, h, m, h, m, dir)
		}
		for i := 0; i < 2; i++ {
			tID := fmt.Sprintf("TRIP_RED_ON_SA_D%d_%d", dir, i)
			mins := i * 15
			h := 1 + mins/60
			m := mins % 60
			fmt.Fprintf(&tripsBuilder, "%s,Red,SER_SA,%d,SHAPE_RED\n", tID, dir)
			fmt.Fprintf(&stopTimesBuilder, "%s,%02d:%02d:00,%02d:%02d:00,STOP_R%d,1\n", tID, h, m, h, m, dir)
		}
		// Uses the after-midnight (hour>=24) representation to exercise that band too.
		fmt.Fprintf(&tripsBuilder, "TRIP_RED_ON_SU_D%d,Red,SER_SU,%d,SHAPE_RED\n", dir, dir)
		fmt.Fprintf(&stopTimesBuilder, "TRIP_RED_ON_SU_D%d,25:30:00,25:30:00,STOP_R%d,1\n", dir, dir)
	}

	// Yellow Line: AM-peak trips only, no overnight trips at all -> overnight
	// should come back nil (no service), not a sentinel.
	for dir := 0; dir < 2; dir++ {
		for i := 0; i < 3; i++ {
			tID := fmt.Sprintf("TRIP_YEL_AM_D%d_%d", dir, i)
			mins := i * 40
			h := 7 + mins/60
			m := mins % 60
			fmt.Fprintf(&tripsBuilder, "%s,Y,SER_WD,%d,SHAPE_YEL\n", tID, dir)
			fmt.Fprintf(&stopTimesBuilder, "%s,%02d:%02d:00,%02d:%02d:00,STOP_Y%d,1\n", tID, h, m, h, m, dir)
		}
	}

	// Non-rail route (route_type 3): should be excluded entirely.
	tripsBuilder.WriteString("TRIP_BUS,99,SER_WD,0,SHAPE_BUS\n")
	stopTimesBuilder.WriteString("TRIP_BUS,07:15:00,07:15:00,STOP_B,1\n")

	shapesCSV := `shape_id,shape_pt_lat,shape_pt_lon,shape_pt_sequence
SHAPE_RED,42.0190,-87.6720,1
SHAPE_RED,41.7220,-87.6250,2
SHAPE_YEL,42.0330,-87.7500,1
SHAPE_YEL,42.0190,-87.7330,2
SHAPE_BUS,41.8800,-87.6300,1
SHAPE_BUS,41.8900,-87.6300,2
`

	files := map[string]string{
		"calendar.txt":   calendarCSV,
		"routes.txt":     routesCSV,
		"trips.txt":      tripsBuilder.String(),
		"stop_times.txt": stopTimesBuilder.String(),
		"shapes.txt":     shapesCSV,
	}
	for name, content := range files {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %q: %v", name, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write content for %q: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	return buf.Bytes()
}

func TestDeriveLineServiceFromZip(t *testing.T) {
	zipBytes := buildRailGTFSZip(t)
	referenceDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	lines, err := deriveLineServiceFromZip(zipBytes, referenceDate)
	if err != nil {
		t.Fatalf("deriveLineServiceFromZip failed: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 rail lines (bus route must be excluded), got %d", len(lines))
	}

	byName := make(map[string]derivedLineService)
	for _, l := range lines {
		byName[l.lineName] = l
	}

	red, ok := byName["Red"]
	if !ok {
		t.Fatalf("missing Red line")
	}
	if red.amPeakHeadwayMinutes != 20.0 {
		t.Errorf("Red amPeakHeadwayMinutes = %.1f, want 20.0", red.amPeakHeadwayMinutes)
	}
	if red.overnightHeadwayMinutes == nil {
		t.Fatalf("Red overnightHeadwayMinutes = nil, want 45.0")
	}
	if *red.overnightHeadwayMinutes != 45.0 {
		t.Errorf("Red overnightHeadwayMinutes = %.1f, want 45.0 (day-weighted: (5*5 + 2*1 + 1*1)/7 = 4.0 trips/night, 180/4.0 = 45.0)", *red.overnightHeadwayMinutes)
	}

	yellow, ok := byName["Yellow"]
	if !ok {
		t.Fatalf("missing Yellow line")
	}
	if yellow.amPeakHeadwayMinutes <= 0 {
		t.Errorf("Yellow amPeakHeadwayMinutes = %.1f, want > 0", yellow.amPeakHeadwayMinutes)
	}
	if yellow.overnightHeadwayMinutes != nil {
		t.Errorf("Yellow overnightHeadwayMinutes = %.1f, want nil (no overnight service)", *yellow.overnightHeadwayMinutes)
	}
}

func TestDeriveLineServiceMissingFiles(t *testing.T) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	_ = zw.Close()

	_, err := deriveLineServiceFromZip(buf.Bytes(), time.Now())
	if err == nil {
		t.Errorf("expected error on empty zip, got nil")
	}
}
