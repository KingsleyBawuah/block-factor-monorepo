package scoring

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

func TestThirdPlaceGridKey_SameCellForNearbyCoordinates(t *testing.T) {
	// Two points ~40m apart (well inside a 400m cell) must bucket identically.
	base := thirdPlaceGridKey(41.8998, -87.6505)
	nearby := thirdPlaceGridKey(41.89983, -87.65053)
	if base != nearby {
		t.Errorf("expected same grid key for nearby coordinates, got %q and %q", base, nearby)
	}
}

func TestThirdPlaceGridKey_DifferentCellForDistantCoordinates(t *testing.T) {
	// Two points ~1.1km apart must land in different cells.
	base := thirdPlaceGridKey(41.8998, -87.6505)
	distant := thirdPlaceGridKey(41.9098, -87.6505)
	if base == distant {
		t.Errorf("expected different grid keys for distant coordinates, got %q for both", base)
	}
}

func TestIsChainCafeBrand(t *testing.T) {
	tests := []struct {
		name     string
		poiName  string
		wantChan bool
	}{
		{"exact chain match", "Starbucks", true},
		{"chain with location suffix", "Dunkin' - Michigan Ave", true},
		{"case-insensitive match", "STARBUCKS RESERVE", true},
		{"independent cafe", "Sip Coffee House", false},
		{"independent cafe with chain-adjacent name", "Metropolis Coffee Company", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isChainCafeBrand(tt.poiName); got != tt.wantChan {
				t.Errorf("isChainCafeBrand(%q) = %v, want %v", tt.poiName, got, tt.wantChan)
			}
		})
	}
}

func TestClassifyThirdPlacePois(t *testing.T) {
	places := []clients.Place{
		{Name: "Starbucks", PrimaryType: "cafe", Lat: 41.9, Lon: -87.65},
		{Name: "Sip Coffee House", PrimaryType: "cafe", Lat: 41.901, Lon: -87.651},
		{Name: "Lincoln Park Fitness", PrimaryType: "gym", Lat: 41.902, Lon: -87.652},
	}

	pois := classifyThirdPlacePois(places)
	if len(pois) != 3 {
		t.Fatalf("expected 3 pois, got %d", len(pois))
	}
	if pois[0].IsIndependentCafe {
		t.Errorf("expected Starbucks to be classified as a chain")
	}
	if !pois[1].IsIndependentCafe {
		t.Errorf("expected Sip Coffee House to be classified as independent")
	}
	if pois[2].IsIndependentCafe {
		t.Errorf("expected a gym to never be flagged as an independent cafe")
	}
}

func TestHaversineDistanceMeters_KnownDistance(t *testing.T) {
	// Chicago Loop to Wrigley Field is a well-known ~8.4 km straight-line distance.
	d := haversineDistanceMeters(41.8789, -87.6359, 41.9484, -87.6553)
	if d < 7800 || d > 9000 {
		t.Errorf("expected distance roughly 7.8-9.0 km, got %f meters", d)
	}
}

func TestHaversineDistanceMeters_SamePointIsZero(t *testing.T) {
	if d := haversineDistanceMeters(41.8781, -87.6298, 41.8781, -87.6298); d != 0 {
		t.Errorf("expected 0 distance for identical points, got %f", d)
	}
}

func poi(name, primaryType string, isIndependent bool, distanceMeters float64) ThirdPlacePoi {
	// Places the POI due north of the fixed origin used by every scenario
	// below, at exactly distanceMeters (roughly -- close enough for these
	// threshold checks given Chicago's latitude).
	const originLat, originLon = 41.8998, -87.6505
	const metersPerDegreeLat = 111_320.0
	return ThirdPlacePoi{
		Name:              name,
		PrimaryType:       primaryType,
		IsIndependentCafe: isIndependent,
		Lat:               originLat + distanceMeters/metersPerDegreeLat,
		Lon:               originLon,
	}
}

func TestCalculateThirdPlaceScore_Scenarios(t *testing.T) {
	const originLat, originLon = 41.8998, -87.6505

	tests := []struct {
		name          string
		pois          []ThirdPlacePoi
		expectedScore float32
		expectedPois  int
	}{
		{
			name:          "no pois",
			pois:          nil,
			expectedScore: 0,
			expectedPois:  0,
		},
		{
			name: "3 independent cafes and 1 gym within 0.4 mi -> 100",
			pois: []ThirdPlacePoi{
				poi("Sip Coffee", "cafe", true, 100),
				poi("Metropolis Coffee", "cafe", true, 200),
				poi("Dark Matter Coffee", "cafe", true, 300),
				poi("Lincoln Park Fitness", "gym", false, 350),
			},
			expectedScore: 100.0,
			expectedPois:  2, // closest cafe + closest fitness
		},
		{
			name: "3 chain cafes and 1 gym within 0.4 mi does not meet the independent-cafe benchmark -> 75 (gym alone)",
			pois: []ThirdPlacePoi{
				poi("Starbucks", "cafe", false, 100),
				poi("Dunkin", "cafe", false, 200),
				poi("Peet's Coffee", "cafe", false, 300),
				poi("Lincoln Park Fitness", "gym", false, 350),
			},
			expectedScore: 75.0,
			expectedPois:  1, // only the gym is a qualifying candidate
		},
		{
			name: "2 independent cafes within 0.5 mi, no gym -> 75",
			pois: []ThirdPlacePoi{
				poi("Sip Coffee", "cafe", true, 400),
				poi("Metropolis Coffee", "cafe", true, 700),
			},
			expectedScore: 75.0,
			expectedPois:  1,
		},
		{
			name: "1 gym within 0.5 mi alone -> 75",
			pois: []ThirdPlacePoi{
				poi("Lincoln Park Fitness", "fitness_center", false, 700),
			},
			expectedScore: 75.0,
			expectedPois:  1,
		},
		{
			name: "1 independent cafe within 0.5 mi -> 40",
			pois: []ThirdPlacePoi{
				poi("Sip Coffee", "cafe", true, 700),
			},
			expectedScore: 40.0,
			expectedPois:  1,
		},
		{
			name: "1 chain cafe within 0.5 mi and nothing else -> 0",
			pois: []ThirdPlacePoi{
				poi("Starbucks", "cafe", false, 200),
			},
			expectedScore: 0.0,
			expectedPois:  0,
		},
		{
			name: "qualifying poi beyond 0.5 mi contributes nothing",
			pois: []ThirdPlacePoi{
				poi("Sip Coffee", "cafe", true, 900),
			},
			expectedScore: 0.0,
			expectedPois:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _, contributing := CalculateThirdPlaceScore(tt.pois, originLat, originLon)
			if score != tt.expectedScore {
				t.Errorf("expected score %v, got %v", tt.expectedScore, score)
			}
			if len(contributing) != tt.expectedPois {
				t.Errorf("expected %d contributing pois, got %d (%+v)", tt.expectedPois, len(contributing), contributing)
			}
		})
	}
}

// mockQueryRow and mockBudgetDB mirror spatial_test's mock pattern
// (services/api/internal/spatial/boundary_test.go), reimplemented locally
// since that mock is unexported in an external package.
type mockQueryRow struct {
	scanFunc func(dest ...any) error
}

func (m *mockQueryRow) Scan(dest ...any) error {
	return m.scanFunc(dest...)
}

type mockBudgetDB struct {
	callCount int
	scanErr   error
}

func (m *mockBudgetDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockQueryRow{
		scanFunc: func(dest ...any) error {
			if m.scanErr != nil {
				return m.scanErr
			}
			m.callCount++
			if ptr, ok := dest[0].(*int); ok {
				*ptr = m.callCount
			}
			return nil
		},
	}
}

func TestDBPlacesBudget_IncrementAndCheck_WithinBudget(t *testing.T) {
	db := &mockBudgetDB{}
	budget := NewDBPlacesBudget(db)

	withinBudget, err := budget.IncrementAndCheck(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !withinBudget {
		t.Errorf("expected withinBudget=true on first call, got false")
	}
}

func TestDBPlacesBudget_IncrementAndCheck_ExceedsCap(t *testing.T) {
	db := &mockBudgetDB{callCount: 4}
	budget := NewDBPlacesBudget(db)

	withinBudget, err := budget.IncrementAndCheck(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !withinBudget {
		t.Errorf("expected withinBudget=true at exactly the cap (5th call), got false")
	}

	withinBudget, err = budget.IncrementAndCheck(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if withinBudget {
		t.Errorf("expected withinBudget=false once past the cap, got true")
	}
}

func TestDBPlacesBudget_IncrementAndCheck_NilDB(t *testing.T) {
	budget := NewDBPlacesBudget(nil)
	_, err := budget.IncrementAndCheck(context.Background(), 5)
	if err == nil {
		t.Fatal("expected error on nil db, got nil")
	}
}

func TestDBPlacesBudget_IncrementAndCheck_QueryError(t *testing.T) {
	db := &mockBudgetDB{scanErr: errors.New("connection failed")}
	budget := NewDBPlacesBudget(db)
	_, err := budget.IncrementAndCheck(context.Background(), 5)
	if err == nil {
		t.Fatal("expected error on query failure, got nil")
	}
}

type mockPlacesQuerier struct {
	callCount int32
	places    []clients.Place
	err       error
	delay     time.Duration
}

func (m *mockPlacesQuerier) SearchNearby(ctx context.Context, _, _, _ float64, _ []string) ([]clients.Place, error) {
	atomic.AddInt32(&m.callCount, 1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.places, nil
}

type mockThirdPlaceCache struct {
	mu    sync.Mutex
	data  map[string]any
	err   error
	setFn func(key string)
}

func (m *mockThirdPlaceCache) Get(_ context.Context, key string, dest any) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[key]
	if !ok {
		return false, nil
	}
	*dest.(*[]ThirdPlacePoi) = v.([]ThirdPlacePoi)
	return true, nil
}

func (m *mockThirdPlaceCache) Set(_ context.Context, key string, value any, _ time.Duration) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = make(map[string]any)
	}
	m.data[key] = value
	if m.setFn != nil {
		m.setFn(key)
	}
	return nil
}

type mockPlacesBudget struct {
	withinBudget bool
	err          error
}

func (m *mockPlacesBudget) IncrementAndCheck(_ context.Context, _ int) (bool, error) {
	return m.withinBudget, m.err
}

func TestThirdPlaceCalculator_Calculate_CacheHit(t *testing.T) {
	cache := &mockThirdPlaceCache{data: map[string]any{
		thirdPlaceGridKey(41.8998, -87.6505): []ThirdPlacePoi{
			{Name: "Sip Coffee", PrimaryType: "cafe", IsIndependentCafe: true, Lat: 41.8998, Lon: -87.6505},
		},
	}}
	places := &mockPlacesQuerier{}
	budget := &mockPlacesBudget{withinBudget: true}

	calc := NewThirdPlaceCalculator(places, cache, budget, 100)
	res, err := calc.Calculate(context.Background(), ScoreParams{Origin: api.Coordinates{Lat: 41.8998, Lon: -87.6505}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Degraded {
		t.Errorf("expected not degraded on cache hit")
	}
	if atomic.LoadInt32(&places.callCount) != 0 {
		t.Errorf("expected no Places API calls on a cache hit, got %d", places.callCount)
	}
	if res.Score == nil || *res.Score != ThirdPlaceScoreMinimal {
		t.Errorf("expected score %v, got %v", ThirdPlaceScoreMinimal, res.Score)
	}
}

func TestThirdPlaceCalculator_Calculate_CacheMissFetchesAndCaches(t *testing.T) {
	cache := &mockThirdPlaceCache{}
	places := &mockPlacesQuerier{places: []clients.Place{
		{Name: "Sip Coffee", PrimaryType: "cafe", Lat: 41.8998, Lon: -87.6505},
	}}
	budget := &mockPlacesBudget{withinBudget: true}

	calc := NewThirdPlaceCalculator(places, cache, budget, 100)
	res, err := calc.Calculate(context.Background(), ScoreParams{Origin: api.Coordinates{Lat: 41.8998, Lon: -87.6505}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Degraded {
		t.Errorf("expected not degraded")
	}
	if atomic.LoadInt32(&places.callCount) != 1 {
		t.Errorf("expected exactly 1 Places API call, got %d", places.callCount)
	}
	if len(cache.data) != 1 {
		t.Errorf("expected the fetch result to be cached, got %d entries", len(cache.data))
	}
}

func TestThirdPlaceCalculator_Calculate_BudgetExhaustedDegradesWithoutError(t *testing.T) {
	cache := &mockThirdPlaceCache{}
	places := &mockPlacesQuerier{places: []clients.Place{{Name: "Sip Coffee", PrimaryType: "cafe"}}}
	budget := &mockPlacesBudget{withinBudget: false}

	calc := NewThirdPlaceCalculator(places, cache, budget, 0)
	res, err := calc.Calculate(context.Background(), ScoreParams{Origin: api.Coordinates{Lat: 41.8998, Lon: -87.6505}})
	if err != nil {
		t.Fatalf("expected budget exhaustion to degrade gracefully, not error: %v", err)
	}
	if !res.Degraded {
		t.Errorf("expected degraded=true once the daily budget is exhausted")
	}
	if atomic.LoadInt32(&places.callCount) != 0 {
		t.Errorf("expected no Places API call once the budget check fails, got %d", places.callCount)
	}
}

func TestThirdPlaceCalculator_Calculate_PlacesErrorDegrades(t *testing.T) {
	cache := &mockThirdPlaceCache{}
	places := &mockPlacesQuerier{err: errors.New("network timeout")}
	budget := &mockPlacesBudget{withinBudget: true}

	calc := NewThirdPlaceCalculator(places, cache, budget, 100)
	res, err := calc.Calculate(context.Background(), ScoreParams{Origin: api.Coordinates{Lat: 41.8998, Lon: -87.6505}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !res.Degraded {
		t.Errorf("expected degraded=true on Places API error")
	}
}

func TestThirdPlaceCalculator_Calculate_NotConfigured(t *testing.T) {
	calc := NewThirdPlaceCalculator(nil, nil, nil, 100)
	res, err := calc.Calculate(context.Background(), ScoreParams{Origin: api.Coordinates{Lat: 41.8998, Lon: -87.6505}})
	if err == nil {
		t.Fatal("expected error for unconfigured calculator, got nil")
	}
	if !res.Degraded {
		t.Errorf("expected degraded=true when unconfigured")
	}
}

func TestThirdPlaceCalculator_Calculate_ConcurrentMissesDedupedBySingleflight(t *testing.T) {
	cache := &mockThirdPlaceCache{}
	places := &mockPlacesQuerier{
		places: []clients.Place{{Name: "Sip Coffee", PrimaryType: "cafe"}},
		delay:  50 * time.Millisecond,
	}
	budget := &mockPlacesBudget{withinBudget: true}

	calc := NewThirdPlaceCalculator(places, cache, budget, 100)

	const concurrency = 10
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for range concurrency {
		go func() {
			defer wg.Done()
			_, err := calc.Calculate(context.Background(), ScoreParams{Origin: api.Coordinates{Lat: 41.8998, Lon: -87.6505}})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&places.callCount); got != 1 {
		t.Errorf("expected exactly 1 Places API call across %d concurrent cache misses on the same cell, got %d", concurrency, got)
	}
}
