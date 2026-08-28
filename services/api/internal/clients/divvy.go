package clients

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	divvyMaxAttempts    = 3
	divvyInitialBackoff = 1 * time.Second

	// DivvyCacheTTL is the in-process cache window DivvyStationsCache holds
	// a fetched station list for (docs/ARCHITECTURE.md §5.4: "Divvy GBFS
	// dock & e-bike availability (5-minute TTL)"). Deliberately longer than
	// the live feed's own advertised ttl (60s, confirmed live) -- we're not
	// one client polling once, but a server serving many concurrent
	// viewers, so trading a little freshness for far fewer upstream calls
	// is the right side of that trade-off for a display-only, unscored map
	// layer.
	DivvyCacheTTL = 5 * time.Minute
)

// divvyBackoffWait mirrors sodaBackoffWait/overpassBackoffWait --
// overridable in tests so retry backoff doesn't cost real wall-clock time.
var divvyBackoffWait = func(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// DivvyStation is a joined, normalized Divvy/Lyft GBFS bike-share dock --
// station_information.json (location/capacity) merged with
// station_status.json (live availability) by station_id.
type DivvyStation struct {
	StationID       string
	Name            string
	Lat             float64
	Lon             float64
	Capacity        int
	BikesAvailable  int
	EBikesAvailable int
	DocksAvailable  int
	// IsRenting/IsReturning report whether the station is currently
	// dispensing/accepting bikes -- a station can be installed but
	// temporarily closed for rebalancing, which capacity numbers alone
	// don't convey.
	IsRenting   bool
	IsReturning bool
}

// gbfsStationInformationResponse and gbfsStationStatusResponse mirror the
// real GBFS 2.3 feed shapes, verified live against
// https://gbfs.lyft.com/gbfs/2.3/chi/en/{station_information,station_status}.json
// rather than assumed from the spec name alone -- is_renting/is_returning
// come back as 1/0 integers, not real JSON booleans.
type gbfsStationInformationResponse struct {
	Data struct {
		Stations []struct {
			StationID string  `json:"station_id"`
			Name      string  `json:"name"`
			Lat       float64 `json:"lat"`
			Lon       float64 `json:"lon"`
			Capacity  int     `json:"capacity"`
		} `json:"stations"`
	} `json:"data"`
}

type gbfsStationStatusResponse struct {
	Data struct {
		Stations []struct {
			StationID          string `json:"station_id"`
			NumBikesAvailable  int    `json:"num_bikes_available"`
			NumEBikesAvailable int    `json:"num_ebikes_available"`
			NumDocksAvailable  int    `json:"num_docks_available"`
			IsRenting          int    `json:"is_renting"`
			IsReturning        int    `json:"is_returning"`
		} `json:"stations"`
	} `json:"data"`
}

// DivvyClient fetches live Divvy/Lyft bike-share dock data from the public
// GBFS 2.3 feed. Results are not cached here -- the caller
// (DivvyStationsCache) owns caching, the same split PlacesClient already
// uses for S_third_place.
type DivvyClient struct {
	httpClient       *http.Client
	stationInfoURL   string
	stationStatusURL string
}

// NewDivvyClient creates a DivvyClient against the given GBFS feed URLs
// (DIVVY_GBFS_STATION_INFO_URL / DIVVY_GBFS_STATION_STATUS_URL).
func NewDivvyClient(stationInfoURL, stationStatusURL string) *DivvyClient {
	return &DivvyClient{
		httpClient:       &http.Client{Timeout: 15 * time.Second},
		stationInfoURL:   stationInfoURL,
		stationStatusURL: stationStatusURL,
	}
}

// FetchStations fetches both GBFS feeds and joins them by station_id. A
// station present in one feed but not the other -- which GBFS's spec
// doesn't actually allow, but real-world feeds can still drift on a given
// poll -- is skipped rather than returned with zeroed-out fields.
func (c *DivvyClient) FetchStations(ctx context.Context) ([]DivvyStation, error) {
	if c.stationInfoURL == "" || c.stationStatusURL == "" {
		return nil, errors.New("clients: divvy GBFS feed URLs not configured")
	}

	var info gbfsStationInformationResponse
	if err := c.fetchJSON(ctx, c.stationInfoURL, &info); err != nil {
		return nil, fmt.Errorf("clients: fetch divvy station information: %w", err)
	}

	var status gbfsStationStatusResponse
	if err := c.fetchJSON(ctx, c.stationStatusURL, &status); err != nil {
		return nil, fmt.Errorf("clients: fetch divvy station status: %w", err)
	}

	type liveStatus struct {
		bikesAvailable  int
		ebikesAvailable int
		docksAvailable  int
		isRenting       bool
		isReturning     bool
	}
	statusByID := make(map[string]liveStatus, len(status.Data.Stations))
	for _, s := range status.Data.Stations {
		statusByID[s.StationID] = liveStatus{
			bikesAvailable:  s.NumBikesAvailable,
			ebikesAvailable: s.NumEBikesAvailable,
			docksAvailable:  s.NumDocksAvailable,
			isRenting:       s.IsRenting == 1,
			isReturning:     s.IsReturning == 1,
		}
	}

	stations := make([]DivvyStation, 0, len(info.Data.Stations))
	for _, i := range info.Data.Stations {
		s, ok := statusByID[i.StationID]
		if !ok {
			continue
		}
		stations = append(stations, DivvyStation{
			StationID:       i.StationID,
			Name:            i.Name,
			Lat:             i.Lat,
			Lon:             i.Lon,
			Capacity:        i.Capacity,
			BikesAvailable:  s.bikesAvailable,
			EBikesAvailable: s.ebikesAvailable,
			DocksAvailable:  s.docksAvailable,
			IsRenting:       s.isRenting,
			IsReturning:     s.isReturning,
		})
	}

	return stations, nil
}

// fetchJSON makes one GET request with retry. Transient failures --
// network errors, 429 rate limiting, 5xx server errors -- are retried with
// exponential backoff (up to divvyMaxAttempts total attempts); other 4xx
// responses fail immediately, mirroring SODAClient.GetJSON.
func (c *DivvyClient) fetchJSON(ctx context.Context, url string, dest any) error {
	backoff := divvyInitialBackoff
	var lastErr error
	for attempt := 1; attempt <= divvyMaxAttempts; attempt++ {
		retryable, err := c.doFetch(ctx, url, dest)
		if err == nil {
			return nil
		}
		lastErr = err

		if !retryable || attempt == divvyMaxAttempts {
			return err
		}

		slog.Warn("Divvy GBFS request failed, retrying",
			slog.String("url", url),
			slog.Int("attempt", attempt),
			slog.Int("max_attempts", divvyMaxAttempts),
			slog.String("error", err.Error()),
		)

		if err := divvyBackoffWait(ctx, backoff); err != nil {
			return err
		}
		backoff *= 2
	}

	return lastErr
}

func (c *DivvyClient) doFetch(ctx context.Context, url string, dest any) (retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("clients: build divvy request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return true, fmt.Errorf("clients: execute divvy request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response already consumed by the time this runs

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return true, fmt.Errorf("clients: divvy request returned status %d: %s", resp.StatusCode, body)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return false, fmt.Errorf("clients: divvy request returned non-retryable status %d: %s", resp.StatusCode, body)
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return false, fmt.Errorf("clients: decode divvy response: %w", err)
	}
	return false, nil
}

// divvyStationFetcher is the subset of DivvyClient DivvyStationsCache
// depends on, kept as an interface so tests can fake it without an HTTP
// server.
type divvyStationFetcher interface {
	FetchStations(ctx context.Context) ([]DivvyStation, error)
}

// DivvyStationsCache wraps a divvyStationFetcher with the in-process,
// 5-minute TTL cache docs/ARCHITECTURE.md §5.4 calls for. On a refresh
// failure with a previous successful fetch already cached, it logs a
// warning and keeps serving the stale value rather than erroring -- a live
// GBFS hiccup shouldn't break a display-only, unscored map layer. Only a
// cold start with no prior successful fetch returns an error.
type DivvyStationsCache struct {
	fetcher divvyStationFetcher
	ttl     time.Duration
	logger  *slog.Logger

	mu        sync.Mutex
	stations  []DivvyStation
	fetchedAt time.Time
}

// NewDivvyStationsCache creates a DivvyStationsCache. ttl is the freshness
// window before a Get triggers a live refresh (DivvyCacheTTL in
// production).
func NewDivvyStationsCache(fetcher divvyStationFetcher, ttl time.Duration, logger *slog.Logger) *DivvyStationsCache {
	if logger == nil {
		logger = slog.Default()
	}
	return &DivvyStationsCache{fetcher: fetcher, ttl: ttl, logger: logger}
}

// Get returns the cached station list, refreshing it first if the cache is
// empty or older than ttl.
func (c *DivvyStationsCache) Get(ctx context.Context) ([]DivvyStation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.stations) > 0 && time.Since(c.fetchedAt) < c.ttl {
		return c.stations, nil
	}

	fresh, err := c.fetcher.FetchStations(ctx)
	if err != nil {
		if len(c.stations) > 0 {
			c.logger.Warn("divvy GBFS refresh failed, serving stale cached data",
				slog.String("error", err.Error()),
				slog.Duration("stale_for", time.Since(c.fetchedAt)),
			)
			return c.stations, nil
		}
		return nil, err
	}

	c.stations = fresh
	c.fetchedAt = time.Now()
	return c.stations, nil
}
