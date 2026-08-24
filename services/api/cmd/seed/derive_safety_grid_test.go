package main

import (
	"math"
	"testing"
)

func TestCalculatePenaltyScore(t *testing.T) {
	tests := []struct {
		name     string
		violent  int
		sexual   int
		property int
		nuisance int
		expected int
	}{
		{
			name:     "zero crimes",
			violent:  0,
			sexual:   0,
			property: 0,
			nuisance: 0,
			expected: 0,
		},
		{
			name:     "violent crime only (5x)",
			violent:  3,
			sexual:   0,
			property: 0,
			nuisance: 0,
			expected: 15,
		},
		{
			name:     "sexual crime only (5x)",
			sexual:   2,
			violent:  0,
			property: 0,
			nuisance: 0,
			expected: 10,
		},
		{
			name:     "property crime only (1x)",
			property: 10,
			violent:  0,
			sexual:   0,
			nuisance: 0,
			expected: 10,
		},
		{
			name:     "nuisance crime only (2x)",
			nuisance: 4,
			violent:  0,
			sexual:   0,
			property: 0,
			expected: 8,
		},
		{
			name:     "combined mixed categories",
			violent:  2,  // 2 * 5 = 10
			sexual:   1,  // 1 * 5 = 5
			property: 20, // 20 * 1 = 20
			nuisance: 5,  // 5 * 2 = 10 -> total 45
			expected: 45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculatePenaltyScore(tt.violent, tt.sexual, tt.property, tt.nuisance)
			if got != tt.expected {
				t.Errorf("calculatePenaltyScore(%d, %d, %d, %d) = %d; want %d",
					tt.violent, tt.sexual, tt.property, tt.nuisance, got, tt.expected)
			}
		})
	}
}

func TestCalculatePercentileRank(t *testing.T) {
	tests := []struct {
		name     string
		scores   []float64
		expected []float64
	}{
		{
			name:     "empty slice",
			scores:   []float64{},
			expected: nil,
		},
		{
			name:     "single score",
			scores:   []float64{50.0},
			expected: []float64{0.0},
		},
		{
			name:     "two distinct scores",
			scores:   []float64{10.0, 30.0},
			expected: []float64{0.0, 100.0},
		},
		{
			name:     "three distinct scores in order",
			scores:   []float64{0.0, 50.0, 100.0},
			expected: []float64{0.0, 50.0, 100.0},
		},
		{
			name:     "five distinct scores unsorted",
			scores:   []float64{40.0, 10.0, 50.0, 20.0, 30.0},
			expected: []float64{75.0, 0.0, 100.0, 25.0, 50.0},
		},
		{
			name:     "tied minimum values share 0 percentile",
			scores:   []float64{0.0, 0.0, 50.0, 100.0, 100.0},
			expected: []float64{0.0, 0.0, 50.0, 75.0, 75.0},
		},
		{
			name:     "all identical scores share 0 percentile",
			scores:   []float64{25.0, 25.0, 25.0, 25.0},
			expected: []float64{0.0, 0.0, 0.0, 0.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculatePercentileRank(tt.scores)
			if len(got) != len(tt.expected) {
				t.Fatalf("len(got) = %d; want %d", len(got), len(tt.expected))
			}
			for i := range got {
				if math.Abs(got[i]-tt.expected[i]) > 0.01 {
					t.Errorf("at index %d: got %.2f, want %.2f", i, got[i], tt.expected[i])
				}
			}
		})
	}
}
