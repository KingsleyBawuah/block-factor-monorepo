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

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
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

	// The two feeds are independent HTTP resources -- fetch them
	// concurrently rather than back-to-back, roughly halving cold-start/
	// cache-miss latency (confirmed live: ~400ms sequential).
	var info gbfsStationInformationResponse
	var status gbfsStationStatusResponse
	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := c.fetchJSON(gCtx, c.stationInfoURL, &info); err != nil {
			return fmt.Errorf("clients: fetch divvy station information: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		if err := c.fetchJSON(gCtx, c.stationStatusURL, &status); err != nil {
			return fmt.Errorf("clients: fetch divvy station status: %w", err)
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
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
		// A canceled/expired context isn't a transient failure worth
		// retrying -- most commonly here, the *other* concurrently-fetched
		// feed (FetchStations' errgroup) failed first and canceled the
		// shared group context; retrying this one would just immediately
		// hit that same cancellation again. Checked via ctx.Err() itself
		// (not errors.Is against context.Canceled/DeadlineExceeded)
		// because errgroup cancels its derived context with the triggering
		// goroutine's own error as the cause, and net/http's client surfaces
		// that cause -- not the plain sentinel -- in the returned error.
		if ctx.Err() != nil {
			return false, fmt.Errorf("clients: execute divvy request: %w", err)
		}
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

	// The real Chicago feed is ~600KB (2050 stations, confirmed live); 10MB
	// is a generous ceiling that guards against unbounded memory allocation
	// on a malformed or runaway upstream response without ever being a
	// real constraint on legitimate data.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 10*1024*1024)).Decode(dest); err != nil {
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
	sf      singleflight.Group

	mu   sync.RWMutex
	data divvyCachedData
}

type divvyCachedData struct {
	stations []DivvyStation
	// fetchedAt is the last *successful* fetch -- used only for the
	// stale-data warning log's "how old is what I'm serving" figure, never
	// for the refresh-cooldown decision (see lastAttempt).
	fetchedAt time.Time
	// lastAttempt is the last fetch attempt, success or failure. Get's
	// freshness check gates on this, not fetchedAt -- otherwise a failed
	// refresh (which leaves fetchedAt untouched) would look "expired"
	// again to the very next caller, triggering another full retry cycle
	// immediately, and the one after that, for as long as the upstream
	// outage lasts.
	lastAttempt time.Time
}

// NewDivvyStationsCache creates a DivvyStationsCache. ttl is the freshness
// window before a Get triggers a live refresh (DivvyCacheTTL in
// production), and also the cooldown before a *failed* refresh is retried.
func NewDivvyStationsCache(fetcher divvyStationFetcher, ttl time.Duration, logger *slog.Logger) *DivvyStationsCache {
	if logger == nil {
		logger = slog.Default()
	}
	return &DivvyStationsCache{fetcher: fetcher, ttl: ttl, logger: logger}
}

// Get returns the cached station list, refreshing it first if the last
// fetch attempt (successful or not) is older than ttl. Concurrent callers
// during a refresh share one singleflight.Group call rather than each
// blocking a mutex across the network round trip or each independently
// hitting the upstream feed. The incoming context is deliberately unused
// (kept in the signature to satisfy the caller-facing shape a data reader
// is expected to have) -- see the singleflight callback below for why the
// shared fetch can't be tied to any one caller's own context.
func (c *DivvyStationsCache) Get(_ context.Context) ([]DivvyStation, error) {
	c.mu.RLock()
	data := c.data
	c.mu.RUnlock()
	if len(data.stations) > 0 && time.Since(data.lastAttempt) < c.ttl {
		return data.stations, nil
	}

	v, err, _ := c.sf.Do("divvy_stations", func() (any, error) {
		// Deliberately not ctx: this fetch is shared across every caller
		// waiting on this singleflight call, so one caller's own
		// request-scoped context being cancelled (e.g. its HTTP client
		// disconnected) must not cancel the shared in-flight fetch for
		// everyone else. DivvyClient's own http.Client timeout (15s) still
		// bounds each attempt.
		fresh, fetchErr := c.fetcher.FetchStations(context.Background())

		c.mu.Lock()
		defer c.mu.Unlock()
		c.data.lastAttempt = time.Now()

		if fetchErr != nil {
			if len(c.data.stations) > 0 {
				c.logger.Warn("divvy GBFS refresh failed, serving stale cached data",
					slog.String("error", fetchErr.Error()),
					slog.Duration("stale_for", time.Since(c.data.fetchedAt)),
				)
				return c.data.stations, nil
			}
			return nil, fetchErr
		}

		c.data.stations = fresh
		c.data.fetchedAt = c.data.lastAttempt
		return c.data.stations, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]DivvyStation), nil
}
