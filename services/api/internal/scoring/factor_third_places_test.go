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

func TestThirdPlaceGridCellCenter_FallsInSameCellAsOrigin(t *testing.T) {
	// Points scattered across a cell must all resolve to a center that maps
	// back to that exact same cell -- otherwise the fill radius calculation
	// (built around "the center is representative of the whole cell") would
	// be centered somewhere outside it.
	points := [][2]float64{
		{41.8998, -87.6505},
		{41.90005, -87.6503},
		{41.8995, -87.6508},
	}

	for _, p := range points {
		wantKey := thirdPlaceGridKey(p[0], p[1])
		centerLat, centerLon := thirdPlaceGridCellCenter(p[0], p[1])
		if gotKey := thirdPlaceGridKey(centerLat, centerLon); gotKey != wantKey {
			t.Errorf("cell center of (%v, %v) maps to a different cell: origin key %q, center key %q", p[0], p[1], wantKey, gotKey)
		}
	}
}

func TestThirdPlaceFillRadiusMeters_CoversQueryRadiusFromAnyPointInCell(t *testing.T) {
	// The whole point of thirdPlaceFillRadiusMeters is that a fetch
	// centered on the cell's center, at that radius, must fully contain
	// every point within ThirdPlaceQueryRadiusMeters of *any* address
	// inside the cell -- including one sitting right at the cell's corner.
	centerLat, centerLon := thirdPlaceGridCellCenter(41.8998, -87.6505)

	// Approximate a worst-case corner address ~283m from the center
	// (roughly a 400m cell's half-diagonal) by walking due north.
	const metersPerDegreeLat = 111_320.0
	cornerLat := centerLat + 283.0/metersPerDegreeLat
	cornerLon := centerLon

	// A POI right at that corner address's own query radius boundary.
	poiLat := cornerLat + ThirdPlaceQueryRadiusMeters/metersPerDegreeLat
	poiLon := cornerLon

	distFromFillCenter := haversineDistanceMeters(centerLat, centerLon, poiLat, poiLon)
	if distFromFillCenter > thirdPlaceFillRadiusMeters {
		t.Errorf("a POI at a corner address's query boundary (%.1fm from cell center) falls outside thirdPlaceFillRadiusMeters (%.1fm)", distFromFillCenter, thirdPlaceFillRadiusMeters)
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
		// A storefront's Google Places display name doesn't always carry
		// the full brand name or its apostrophe -- these must still match.
		{"chain brand without apostrophe", "Peets Coffee", true},
		{"chain brand as bare storefront name, no suffix", "Philz", true},
		{"chain brand with possessive apostrophe intact", "Peet's Coffee & Tea", true},
		// Chains flagged as gaps in code review.
		{"Panera Bread", "Panera Bread", true},
		{"Pret A Manger full name", "Pret A Manger", true},
		{"Einstein Bros Bagels", "Einstein Bros. Bagels", true},
		{"Au Bon Pain", "Au Bon Pain - Union Station", true},
		{"Capital One Cafe", "Capital One Cafe", true},
		// "pret" alone is a substring of ordinary English words -- matching
		// only the full "pret a manger" brand avoids flagging real
		// independent cafes that happen to contain it.
		{"independent cafe containing 'pret' as a substring", "Pretty Petals Cafe", false},
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
		{Name: "Metropolis Coffee", PrimaryType: "coffee_shop", Lat: 41.9015, Lon: -87.6515},
		{Name: "Lincoln Park Fitness", PrimaryType: "gym", Lat: 41.902, Lon: -87.652},
		{Name: "Yoga Six", PrimaryType: "yoga_studio", Lat: 41.903, Lon: -87.653},
	}

	pois := classifyThirdPlacePois(places)
	if len(pois) != 5 {
		t.Fatalf("expected 5 pois, got %d", len(pois))
	}
	if pois[0].IsIndependentCafe {
		t.Errorf("expected Starbucks to be classified as a chain")
	}
	if !pois[1].IsIndependentCafe {
		t.Errorf("expected Sip Coffee House to be classified as independent")
	}
	if !pois[2].IsIndependentCafe {
		t.Errorf("expected an independent coffee_shop to be classified as independent -- coffee_shop is a distinct Places type from cafe, not a gap")
	}
	if pois[3].IsIndependentCafe {
		t.Errorf("expected a gym to never be flagged as an independent cafe")
	}
	if pois[4].IsIndependentCafe {
		t.Errorf("expected a yoga studio to never be flagged as an independent cafe")
	}
}

func TestIsCafeType(t *testing.T) {
	if !isCafeType("cafe") || !isCafeType("coffee_shop") {
		t.Errorf("expected both cafe and coffee_shop to be recognized as cafe types")
	}
	if isCafeType("gym") {
		t.Errorf("expected gym to not be recognized as a cafe type")
	}
}

func TestIsFitnessType(t *testing.T) {
	for _, tp := range []string{"gym", "fitness_center", "yoga_studio"} {
		if !isFitnessType(tp) {
			t.Errorf("expected %q to be recognized as a fitness type", tp)
		}
	}
	if isFitnessType("cafe") {
		t.Errorf("expected cafe to not be recognized as a fitness type")
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
		{
			name: "3 independent cafes and 1 gym exactly at the 0.4 mi boundary (inclusive) -> 100",
			pois: []ThirdPlacePoi{
				poi("Sip Coffee", "cafe", true, ThirdPlaceCloseThresholdMeters),
				poi("Metropolis Coffee", "cafe", true, ThirdPlaceCloseThresholdMeters),
				poi("Dark Matter Coffee", "cafe", true, ThirdPlaceCloseThresholdMeters),
				poi("Lincoln Park Fitness", "gym", false, ThirdPlaceCloseThresholdMeters),
			},
			expectedScore: 100.0,
			expectedPois:  2,
		},
		{
			name: "3rd independent cafe just past the 0.4 mi boundary misses the 100 pt tier -> 75",
			pois: []ThirdPlacePoi{
				poi("Sip Coffee", "cafe", true, ThirdPlaceCloseThresholdMeters),
				poi("Metropolis Coffee", "cafe", true, ThirdPlaceCloseThresholdMeters),
				poi("Dark Matter Coffee", "cafe", true, ThirdPlaceCloseThresholdMeters+1),
				poi("Lincoln Park Fitness", "gym", false, ThirdPlaceCloseThresholdMeters),
			},
			expectedScore: 75.0,
			expectedPois:  2,
		},
		{
			name: "1 independent cafe exactly at the 0.5 mi boundary (inclusive) -> 40",
			pois: []ThirdPlacePoi{
				poi("Sip Coffee", "cafe", true, ThirdPlaceQueryRadiusMeters),
			},
			expectedScore: 40.0,
			expectedPois:  1,
		},
		{
			name: "1 independent cafe just past the 0.5 mi boundary -> 0",
			pois: []ThirdPlacePoi{
				poi("Sip Coffee", "cafe", true, ThirdPlaceQueryRadiusMeters+1),
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
	mu             sync.Mutex
	callCount      int32
	places         []clients.Place
	err            error
	delay          time.Duration
	capturedLat    float64
	capturedLon    float64
	capturedRadius float64
}

func (m *mockPlacesQuerier) SearchNearby(ctx context.Context, lat, lon, radiusMeters float64, _ []string) ([]clients.Place, error) {
	atomic.AddInt32(&m.callCount, 1)
	m.mu.Lock()
	m.capturedLat, m.capturedLon, m.capturedRadius = lat, lon, radiusMeters
	m.mu.Unlock()
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

func TestThirdPlaceCalculator_Calculate_FillsFromGridCellCenterNotRequestOrigin(t *testing.T) {
	// Two different addresses sharing a grid cell must produce the exact
	// same cache-fill request -- otherwise a later address near the edge
	// of the cell would silently reuse a fetch that never covered its own
	// query radius (the bug flagged in code review).
	origin := api.Coordinates{Lat: 41.8998, Lon: -87.6505}
	wantCenterLat, wantCenterLon := thirdPlaceGridCellCenter(origin.Lat, origin.Lon)

	cache := &mockThirdPlaceCache{}
	places := &mockPlacesQuerier{places: []clients.Place{{Name: "Sip Coffee", PrimaryType: "cafe"}}}
	budget := &mockPlacesBudget{withinBudget: true}

	calc := NewThirdPlaceCalculator(places, cache, budget, 100)
	if _, err := calc.Calculate(context.Background(), ScoreParams{Origin: origin}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	places.mu.Lock()
	defer places.mu.Unlock()
	if places.capturedLat != wantCenterLat || places.capturedLon != wantCenterLon {
		t.Errorf("expected fetch centered on grid cell center (%v, %v), got (%v, %v) -- request origin was (%v, %v)",
			wantCenterLat, wantCenterLon, places.capturedLat, places.capturedLon, origin.Lat, origin.Lon)
	}
	if places.capturedRadius != thirdPlaceFillRadiusMeters {
		t.Errorf("expected fetch radius %v, got %v", thirdPlaceFillRadiusMeters, places.capturedRadius)
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

// TestThirdPlacePoiResolver_SharedAcrossConsumers_DedupesFetch is the whole
// point of extracting ThirdPlacePoiResolver out of ThirdPlaceCalculator
// (docs/IMPLEMENTATION_PHASES.md Phase 4 deliverable 6): a
// ThirdPlaceCalculator and a second, independent consumer (standing in for
// the nearby-places endpoint) sharing one resolver instance must dedupe a
// concurrent cache miss for the same grid cell into a single Places API
// call, not one each.
func TestThirdPlacePoiResolver_SharedAcrossConsumers_DedupesFetch(t *testing.T) {
	cache := &mockThirdPlaceCache{}
	places := &mockPlacesQuerier{
		places: []clients.Place{{Name: "Sip Coffee", PrimaryType: "cafe"}},
		delay:  50 * time.Millisecond,
	}
	budget := &mockPlacesBudget{withinBudget: true}

	resolver := NewThirdPlacePoiResolver(places, cache, budget, 100)
	calc := NewThirdPlaceCalculatorFromResolver(resolver)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := calc.Calculate(context.Background(), ScoreParams{Origin: api.Coordinates{Lat: 41.8998, Lon: -87.6505}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if _, _, err := resolver.Resolve(context.Background(), 41.8998, -87.6505); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}()
	wg.Wait()

	if got := atomic.LoadInt32(&places.callCount); got != 1 {
		t.Errorf("expected exactly 1 Places API call shared between both consumers, got %d", got)
	}
}
