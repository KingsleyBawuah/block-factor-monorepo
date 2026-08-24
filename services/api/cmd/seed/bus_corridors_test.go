package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestParseTimeSeconds(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		valid    bool
	}{
		{"07:00:00", 25200, true},
		{"08:30:15", 30615, true},
		{"09:00:00", 32400, true},
		{"24:15:00", 87300, true},
		{"00:00:00", 0, true},
		{"invalid", 0, false},
		{"12:34", 0, false},
		{"ab:cd:ef", 0, false},
		{"", 0, false},
	}

	for _, tc := range tests {
		got, ok := parseTimeSeconds(tc.input)
		if ok != tc.valid {
			t.Errorf("parseTimeSeconds(%q) ok = %v, want %v", tc.input, ok, tc.valid)
		}
		if ok && got != tc.expected {
			t.Errorf("parseTimeSeconds(%q) = %d, want %d", tc.input, got, tc.expected)
		}
	}
}

// buildSyntheticGTFSZip generates an in-memory zip archive with minimal valid GTFS files for testing.
func buildSyntheticGTFSZip(t *testing.T) []byte {
	t.Helper()

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	calendarCSV := `service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date
SER_WD,1,1,1,1,1,0,0,20260101,20261231
SER_WE,0,0,0,0,0,1,1,20260101,20261231
`
	routesCSV := `route_id,route_short_name,route_long_name,route_type
R_FREQ,66,Chicago,3
R_INFREQ,99,Infrequent Local,3
R_NIGHT,N5,Night Owl,3
R_TRAIN,Red,Red Line,1
`

	// R_FREQ: 15 trips in dir 0, 15 trips in dir 1 -> total 30 trips -> avg 15 -> headway = 120/15 = 8.0 min (qualifies <= 10m)
	// R_INFREQ: 2 trips in dir 0, 2 trips in dir 1 -> total 4 trips -> avg 2 -> headway = 120/2 = 60.0 min
	// R_NIGHT: 0 weekday AM rush trips -> headway = 999.00 min sentinel
	var tripsBuilder strings.Builder
	tripsBuilder.WriteString("trip_id,route_id,service_id,direction_id,shape_id\n")

	var stopTimesBuilder strings.Builder
	stopTimesBuilder.WriteString("trip_id,arrival_time,departure_time,stop_id,stop_sequence\n")

	tripCounter := 1

	// Add 15 weekday AM-peak trips for R_FREQ in Dir 0 (every 8 mins from 07:00 to 08:52)
	for i := 0; i < 15; i++ {
		tID := fmt.Sprintf("TRIP_FREQ_D0_%d", tripCounter)
		tripCounter++
		fmt.Fprintf(&tripsBuilder, "%s,R_FREQ,SER_WD,0,SHAPE_FREQ_1\n", tID)
		mins := i * 8
		h := 7 + mins/60
		m := mins % 60
		fmt.Fprintf(&stopTimesBuilder, "%s,%02d:%02d:00,%02d:%02d:00,STOP_1,1\n", tID, h, m, h, m)
	}

	// Add 15 weekday AM-peak trips for R_FREQ in Dir 1
	for i := 0; i < 15; i++ {
		tID := fmt.Sprintf("TRIP_FREQ_D1_%d", tripCounter)
		tripCounter++
		fmt.Fprintf(&tripsBuilder, "%s,R_FREQ,SER_WD,1,SHAPE_FREQ_2\n", tID)
		mins := i * 8
		h := 7 + mins/60
		m := mins % 60
		fmt.Fprintf(&stopTimesBuilder, "%s,%02d:%02d:00,%02d:%02d:00,STOP_2,1\n", tID, h, m, h, m)
	}

	// Add weekend trip for R_FREQ (should be ignored by weekday filter)
	tripsBuilder.WriteString("TRIP_FREQ_WE,R_FREQ,SER_WE,0,SHAPE_FREQ_1\n")
	stopTimesBuilder.WriteString("TRIP_FREQ_WE,07:30:00,07:30:00,STOP_1,1\n")

	// Add 2 weekday AM-peak trips for R_INFREQ in Dir 0 and 2 in Dir 1 (headway 60m)
	for i := 0; i < 2; i++ {
		tID0 := fmt.Sprintf("TRIP_INFREQ_D0_%d", tripCounter)
		tripCounter++
		fmt.Fprintf(&tripsBuilder, "%s,R_INFREQ,SER_WD,0,SHAPE_INFREQ\n", tID0)
		fmt.Fprintf(&stopTimesBuilder, "%s,07:%02d:00,07:%02d:00,STOP_3,1\n", tID0, i*45, i*45)

		tID1 := fmt.Sprintf("TRIP_INFREQ_D1_%d", tripCounter)
		tripCounter++
		fmt.Fprintf(&tripsBuilder, "%s,R_INFREQ,SER_WD,1,SHAPE_INFREQ\n", tID1)
		fmt.Fprintf(&stopTimesBuilder, "%s,07:%02d:00,07:%02d:00,STOP_4,1\n", tID1, i*45, i*45)
	}

	// Add night trip for R_NIGHT (02:30:00 - outside AM peak window)
	tripsBuilder.WriteString("TRIP_NIGHT_1,R_NIGHT,SER_WD,0,SHAPE_NIGHT\n")
	stopTimesBuilder.WriteString("TRIP_NIGHT_1,02:30:00,02:30:00,STOP_N,1\n")

	// Add train trips (route_type = 1, should be ignored)
	tripsBuilder.WriteString("TRIP_TRAIN,R_TRAIN,SER_WD,0,SHAPE_TRAIN\n")
	stopTimesBuilder.WriteString("TRIP_TRAIN,07:15:00,07:15:00,STOP_5,1\n")

	shapesCSV := `shape_id,shape_pt_lat,shape_pt_lon,shape_pt_sequence
SHAPE_FREQ_1,41.8950,-87.6500,1
SHAPE_FREQ_1,41.8950,-87.6600,2
SHAPE_FREQ_1,41.8950,-87.6700,3
SHAPE_FREQ_2,41.8960,-87.6700,1
SHAPE_FREQ_2,41.8960,-87.6500,2
SHAPE_INFREQ,41.7000,-87.6000,1
SHAPE_INFREQ,41.7100,-87.6000,2
SHAPE_NIGHT,41.7500,-87.5800,1
SHAPE_NIGHT,41.7600,-87.5800,2
SHAPE_TRAIN,41.9000,-87.6200,1
SHAPE_TRAIN,41.8800,-87.6200,2
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

func TestDeriveBusCorridorsFromZip(t *testing.T) {
	zipBytes := buildSyntheticGTFSZip(t)

	corridors, err := deriveBusCorridorsFromZip(zipBytes)
	if err != nil {
		t.Fatalf("deriveBusCorridorsFromZip failed: %v", err)
	}

	// Should derive all 3 bus routes (R_FREQ, R_INFREQ, R_NIGHT), excluding R_TRAIN
	if len(corridors) != 3 {
		t.Fatalf("expected 3 bus corridors, got %d", len(corridors))
	}

	corridorMap := make(map[string]derivedCorridor)
	for _, c := range corridors {
		corridorMap[c.routeID] = c
	}

	// Check R_FREQ (8.0m headway)
	cFreq, ok := corridorMap["R_FREQ"]
	if !ok {
		t.Fatalf("missing R_FREQ")
	}
	if cFreq.routeName != "66 - Chicago" {
		t.Errorf("expected routeName '66 - Chicago', got %q", cFreq.routeName)
	}
	if cFreq.meanHeadway != 8.0 {
		t.Errorf("expected meanHeadway 8.0, got %f", cFreq.meanHeadway)
	}
	if !strings.HasPrefix(cFreq.wkt, "MULTILINESTRING(") {
		t.Errorf("expected WKT to start with 'MULTILINESTRING(', got %q", cFreq.wkt)
	}

	// Check R_INFREQ (60.0m headway)
	cInfreq, ok := corridorMap["R_INFREQ"]
	if !ok {
		t.Fatalf("missing R_INFREQ")
	}
	if cInfreq.meanHeadway != 60.0 {
		t.Errorf("expected meanHeadway 60.0, got %f", cInfreq.meanHeadway)
	}

	// Check R_NIGHT (999.0m sentinel headway)
	cNight, ok := corridorMap["R_NIGHT"]
	if !ok {
		t.Fatalf("missing R_NIGHT")
	}
	if cNight.meanHeadway != noAMPeakHeadwaySentinel {
		t.Errorf("expected sentinel headway %f, got %f", noAMPeakHeadwaySentinel, cNight.meanHeadway)
	}
}

func TestDeriveBusCorridorsMissingFiles(t *testing.T) {
	// Empty zip
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	_ = zw.Close()

	_, err := deriveBusCorridorsFromZip(buf.Bytes())
	if err == nil {
		t.Errorf("expected error on empty zip, got nil")
	}
}
