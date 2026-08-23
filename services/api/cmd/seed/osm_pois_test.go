package main

import "testing"

func TestClassifyOSMPOI(t *testing.T) {
	tests := []struct {
		name string
		tags map[string]string
		want string
		ok   bool
	}{
		{"supermarket", map[string]string{"shop": "supermarket"}, "supermarket", true},
		{"convenience", map[string]string{"shop": "convenience"}, "convenience", true},
		{"pharmacy", map[string]string{"amenity": "pharmacy"}, "pharmacy", true},
		{"pharmacy takes priority over an unrelated shop tag", map[string]string{"amenity": "pharmacy", "shop": "chemist"}, "pharmacy", true},
		{"unrelated shop value", map[string]string{"shop": "bakery"}, "", false},
		{"unrelated amenity value", map[string]string{"amenity": "restaurant"}, "", false},
		{"no relevant tags", map[string]string{"name": "Some Place"}, "", false},
		{"empty tags", map[string]string{}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := classifyOSMPOI(tt.tags)
			if ok != tt.ok {
				t.Fatalf("classifyOSMPOI(%v) ok = %v, want %v", tt.tags, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("classifyOSMPOI(%v) = %q, want %q", tt.tags, got, tt.want)
			}
		})
	}
}
