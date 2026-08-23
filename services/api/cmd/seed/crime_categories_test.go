package main

import "testing"

func TestClassifyCrime_PrimaryTypes(t *testing.T) {
	tests := []struct {
		primaryType string
		want        string
	}{
		// Violent (5x)
		{"HOMICIDE", "violent"},
		{"ASSAULT", "violent"},
		{"BATTERY", "violent"},
		{"ROBBERY", "violent"},
		{"KIDNAPPING", "violent"},
		{"HUMAN TRAFFICKING", "violent"},
		{"DOMESTIC VIOLENCE", "violent"},
		{"WEAPONS VIOLATION", "violent"},
		{"CONCEALED CARRY LICENSE VIOLATION", "violent"},
		{"STALKING", "violent"},
		{"INTIMIDATION", "violent"},
		// Sexual (5x) -- including the legacy spelling
		{"CRIMINAL SEXUAL ASSAULT", "sexual"},
		{"CRIM SEXUAL ASSAULT", "sexual"},
		{"SEX OFFENSE", "sexual"},
		// Property (1x)
		{"THEFT", "property"},
		{"BURGLARY", "property"},
		{"MOTOR VEHICLE THEFT", "property"},
		{"CRIMINAL DAMAGE", "property"},
		{"ARSON", "property"},
		{"CRIMINAL TRESPASS", "property"},
		// Nuisance (2x)
		{"NARCOTICS", "nuisance"},
		{"OTHER NARCOTIC VIOLATION", "nuisance"},
		{"PROSTITUTION", "nuisance"},
		{"PUBLIC PEACE VIOLATION", "nuisance"},
		{"PUBLIC INDECENCY", "nuisance"},
		{"OBSCENITY", "nuisance"},
		{"INTERFERENCE WITH PUBLIC OFFICER", "nuisance"},
		// Excluded
		{"GAMBLING", categoryExcluded},
		{"NON-CRIMINAL", categoryExcluded},
		{"LIQUOR LAW VIOLATION", categoryExcluded},
		{"DECEPTIVE PRACTICE", categoryExcluded},
		{"RITUALISM", categoryExcluded},
	}

	for _, tt := range tests {
		t.Run(tt.primaryType, func(t *testing.T) {
			got, ok := classifyCrime(tt.primaryType, "0000")
			if !ok {
				t.Fatalf("classifyCrime(%q, _) = not ok, want ok with category %q", tt.primaryType, tt.want)
			}
			if got != tt.want {
				t.Errorf("classifyCrime(%q, _) = %q, want %q", tt.primaryType, got, tt.want)
			}
		})
	}
}

func TestClassifyCrime_CatchAllIUCRSubCodes(t *testing.T) {
	tests := []struct {
		primaryType string
		iucr        string
		want        string
	}{
		// OTHER OFFENSE sub-codes
		{"OTHER OFFENSE", "2820", "violent"},        // telephone threat
		{"OTHER OFFENSE", "5110", "violent"},        // gun offender registration
		{"OTHER OFFENSE", "5121", "violent"},        // arsonist registration
		{"OTHER OFFENSE", "4650", "sexual"},         // sex offender registration
		{"OTHER OFFENSE", "502P", "property"},       // stolen temp plate
		{"OTHER OFFENSE", "5011", categoryExcluded}, // license violation
		// OFFENSE INVOLVING CHILDREN sub-codes
		{"OFFENSE INVOLVING CHILDREN", "1750", "violent"},        // child abuse
		{"OFFENSE INVOLVING CHILDREN", "1751", "sexual"},         // sexual abuse by family member
		{"OFFENSE INVOLVING CHILDREN", "1720", categoryExcluded}, // contribute to delinquency
		{"OFFENSE INVOLVING CHILDREN", "1791", categoryExcluded}, // harbor runaway
	}

	for _, tt := range tests {
		t.Run(tt.primaryType+"/"+tt.iucr, func(t *testing.T) {
			got, ok := classifyCrime(tt.primaryType, tt.iucr)
			if !ok {
				t.Fatalf("classifyCrime(%q, %q) = not ok, want ok with category %q", tt.primaryType, tt.iucr, tt.want)
			}
			if got != tt.want {
				t.Errorf("classifyCrime(%q, %q) = %q, want %q", tt.primaryType, tt.iucr, got, tt.want)
			}
		})
	}
}

func TestClassifyCrime_UnrecognizedPrimaryType(t *testing.T) {
	_, ok := classifyCrime("SOMETHING NEW CPD JUST INVENTED", "0000")
	if ok {
		t.Fatal("expected classifyCrime to report an unrecognized primary_type as not ok")
	}
}

func TestClassifyCrime_UnrecognizedCatchAllIUCRCode(t *testing.T) {
	// A brand-new iucr sub-code under a catch-all primary_type that isn't
	// in crimeIUCRSubCategory yet -- must not silently default to a
	// category or to excluded; must be reported unrecognized so it gets
	// surfaced (see ingestCrimes' unrecognized-classification warning) and
	// docs/DATA_SOURCES.md gets extended deliberately.
	_, ok := classifyCrime("OTHER OFFENSE", "9999")
	if ok {
		t.Fatal("expected classifyCrime to report an unrecognized iucr sub-code as not ok")
	}
}

func TestClassifyCrime_NonCatchAllPrimaryTypeIgnoresIUCR(t *testing.T) {
	// For an ordinary (non-catch-all) primary_type, classification must
	// depend only on primary_type -- the iucr value must never matter.
	got1, ok1 := classifyCrime("BATTERY", "0000")
	got2, ok2 := classifyCrime("BATTERY", "9999")
	if !ok1 || !ok2 || got1 != got2 {
		t.Fatalf("expected BATTERY to classify identically regardless of iucr, got (%q, %v) vs (%q, %v)", got1, ok1, got2, ok2)
	}
}

// Every crimeIUCRSubCategory entry should only ever be reachable through
// one of the documented catch-all primary_types -- this guards against a
// future edit accidentally adding a primary_type to
// crimePrimaryTypeCategory that collides with a sub-code key in a
// confusing way.
func TestClassifyCrime_CatchAllPrimaryTypesAreExactlyDocumented(t *testing.T) {
	want := map[string]bool{
		"OTHER OFFENSE":              true,
		"OFFENSE INVOLVING CHILDREN": true,
	}
	if len(crimeCatchAllPrimaryTypes) != len(want) {
		t.Fatalf("crimeCatchAllPrimaryTypes = %v, want %v", crimeCatchAllPrimaryTypes, want)
	}
	for k := range want {
		if !crimeCatchAllPrimaryTypes[k] {
			t.Errorf("expected %q to be a catch-all primary type", k)
		}
	}
}
