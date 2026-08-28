package clients

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func withNoDivvyBackoffWait(t *testing.T) {
	t.Helper()
	original := divvyBackoffWait
	divvyBackoffWait = func(ctx context.Context, _ time.Duration) error {
		return ctx.Err()
	}
	t.Cleanup(func() { divvyBackoffWait = original })
}

const fakeStationInfoJSON = `{
	"data": {"stations": [
		{"station_id": "1", "name": "California Blvd & 29th St", "lat": 41.84024, "lon": -87.69413, "capacity": 8},
		{"station_id": "2", "name": "Kedzie Ave & 54th Pl", "lat": 41.794357, "lon": -87.703467, "capacity": 11},
		{"station_id": "3", "name": "Orphaned Info-Only Station", "lat": 41.9, "lon": -87.65, "capacity": 5}
	]}
}`

const fakeStationStatusJSON = `{
	"data": {"stations": [
		{"station_id": "1", "num_bikes_available": 2, "num_ebikes_available": 1, "num_docks_available": 5, "is_renting": 1, "is_returning": 1},
		{"station_id": "2", "num_bikes_available": 0, "num_ebikes_available": 0, "num_docks_available": 11, "is_renting": 0, "is_returning": 1},
		{"station_id": "4", "num_bikes_available": 3, "num_ebikes_available": 0, "num_docks_available": 2, "is_renting": 1, "is_returning": 1}
	]}
}`

func TestDivvyClient_FetchStations_JoinsInfoAndStatusByStationID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			_, _ = w.Write([]byte(fakeStationInfoJSON))
		case "/status":
			_, _ = w.Write([]byte(fakeStationStatusJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := NewDivvyClient(srv.URL+"/info", srv.URL+"/status")
	stations, err := c.FetchStations(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Station "3" (info-only) and "4" (status-only) must be excluded --
	// GBFS's spec doesn't allow that split, but a real feed can still
	// drift between two separately-fetched files.
	if len(stations) != 2 {
		t.Fatalf("expected 2 joined stations, got %d: %+v", len(stations), stations)
	}

	byID := map[string]DivvyStation{}
	for _, s := range stations {
		byID[s.StationID] = s
	}

	s1, ok := byID["1"]
	if !ok {
		t.Fatal("expected station 1 in the joined result")
	}
	if s1.Name != "California Blvd & 29th St" || s1.Lat != 41.84024 || s1.Lon != -87.69413 || s1.Capacity != 8 {
		t.Errorf("unexpected station 1 info fields: %+v", s1)
	}
	if s1.BikesAvailable != 2 || s1.EBikesAvailable != 1 || s1.DocksAvailable != 5 {
		t.Errorf("unexpected station 1 status fields: %+v", s1)
	}
	// is_renting: 1 / is_returning: 1 -> both true.
	if !s1.IsRenting || !s1.IsReturning {
		t.Errorf("expected station 1 IsRenting=true IsReturning=true, got %+v", s1)
	}

	s2, ok := byID["2"]
	if !ok {
		t.Fatal("expected station 2 in the joined result")
	}
	// is_renting: 0 -> false (a station can be installed but temporarily
	// closed for rebalancing -- the real-world case this field exists for).
	if s2.IsRenting {
		t.Errorf("expected station 2 IsRenting=false (is_renting: 0), got true")
	}
	if !s2.IsReturning {
		t.Errorf("expected station 2 IsReturning=true (is_returning: 1), got false")
	}

	if _, ok := byID["3"]; ok {
		t.Error("expected info-only station 3 to be excluded")
	}
	if _, ok := byID["4"]; ok {
		t.Error("expected status-only station 4 to be excluded")
	}
}

func TestDivvyClient_FetchStations_NotConfigured(t *testing.T) {
	c := NewDivvyClient("", "")
	if _, err := c.FetchStations(context.Background()); err == nil {
		t.Fatal("expected an error for unconfigured feed URLs, got nil")
	}
}

func TestDivvyClient_FetchStations_InfoFetchErrorPropagates(t *testing.T) {
	withNoDivvyBackoffWait(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			_, _ = w.Write([]byte(fakeStationStatusJSON))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewDivvyClient(srv.URL+"/info", srv.URL+"/status")
	if _, err := c.FetchStations(context.Background()); err == nil {
		t.Fatal("expected the station information fetch error to propagate, got nil")
	}
}

func TestDivvyClient_FetchStations_RetriesTransientFailures(t *testing.T) {
	withNoDivvyBackoffWait(t)
	var infoAttempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			if infoAttempts.Add(1) < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(fakeStationInfoJSON))
		case "/status":
			_, _ = w.Write([]byte(fakeStationStatusJSON))
		}
	}))
	defer srv.Close()

	c := NewDivvyClient(srv.URL+"/info", srv.URL+"/status")
	stations, err := c.FetchStations(context.Background())
	if err != nil {
		t.Fatalf("expected retry to eventually succeed, got error: %v", err)
	}
	if len(stations) != 2 {
		t.Fatalf("expected 2 joined stations after retry, got %d", len(stations))
	}
	if got := infoAttempts.Load(); got != 3 {
		t.Errorf("expected exactly 3 attempts, got %d", got)
	}
}

func TestDivvyClient_FetchStations_NonRetryableStatusFailsImmediately(t *testing.T) {
	withNoDivvyBackoffWait(t)
	var infoAttempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/info" {
			infoAttempts.Add(1)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(fakeStationStatusJSON))
	}))
	defer srv.Close()

	c := NewDivvyClient(srv.URL+"/info", srv.URL+"/status")
	if _, err := c.FetchStations(context.Background()); err == nil {
		t.Fatal("expected an error, got nil")
	}
	if got := infoAttempts.Load(); got != 1 {
		t.Errorf("expected exactly 1 attempt for a non-retryable status, got %d", got)
	}
}

// mockDivvyFetcher lets DivvyStationsCache tests control exactly what
// FetchStations returns/errors on each call, without a real HTTP server.
type mockDivvyFetcher struct {
	callCount int
	stations  []DivvyStation
	err       error
}

func (m *mockDivvyFetcher) FetchStations(_ context.Context) ([]DivvyStation, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.stations, nil
}

func TestDivvyStationsCache_Get_FetchesOnceThenServesFromCache(t *testing.T) {
	fetcher := &mockDivvyFetcher{stations: []DivvyStation{{StationID: "1", Name: "Test"}}}
	cache := NewDivvyStationsCache(fetcher, time.Hour, nil)

	first, err := cache.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := cache.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fetcher.callCount != 1 {
		t.Errorf("expected exactly 1 fetch within the TTL window, got %d", fetcher.callCount)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Errorf("expected both calls to return the cached station, got %+v / %+v", first, second)
	}
}

func TestDivvyStationsCache_Get_RefetchesAfterTTLExpires(t *testing.T) {
	fetcher := &mockDivvyFetcher{stations: []DivvyStation{{StationID: "1"}}}
	cache := NewDivvyStationsCache(fetcher, time.Millisecond, nil)

	if _, err := cache.Get(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := cache.Get(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fetcher.callCount != 2 {
		t.Errorf("expected 2 fetches after the TTL expired, got %d", fetcher.callCount)
	}
}

func TestDivvyStationsCache_Get_ServesStaleDataOnRefreshFailure(t *testing.T) {
	fetcher := &mockDivvyFetcher{stations: []DivvyStation{{StationID: "1", Name: "Still here"}}}
	cache := NewDivvyStationsCache(fetcher, time.Millisecond, nil)

	if _, err := cache.Get(context.Background()); err != nil {
		t.Fatalf("unexpected error priming the cache: %v", err)
	}

	fetcher.err = errors.New("upstream GBFS feed unreachable")
	time.Sleep(5 * time.Millisecond)

	stations, err := cache.Get(context.Background())
	if err != nil {
		t.Fatalf("expected a refresh failure to serve stale data, not error, got: %v", err)
	}
	if len(stations) != 1 || stations[0].Name != "Still here" {
		t.Errorf("expected the stale cached station to still be served, got %+v", stations)
	}
}

func TestDivvyStationsCache_Get_ColdStartFailurePropagatesError(t *testing.T) {
	fetcher := &mockDivvyFetcher{err: errors.New("upstream GBFS feed unreachable")}
	cache := NewDivvyStationsCache(fetcher, time.Hour, nil)

	if _, err := cache.Get(context.Background()); err == nil {
		t.Fatal("expected a cold-start fetch failure (no prior cached data) to propagate as an error")
	}
}
