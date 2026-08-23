package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// overpassBaseURL is a var, not a const, so tests can point it at an
// httptest.Server instead of the real Overpass API.
var overpassBaseURL = "https://overpass-api.de/api/interpreter"

const (
	overpassMaxAttempts    = 3
	overpassInitialBackoff = 2 * time.Second
)

// overpassBackoffWait mirrors sodaBackoffWait -- overridable in tests so
// retry backoff doesn't cost real wall-clock time.
var overpassBackoffWait = func(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// OverpassClient queries the Overpass API -- a live, purpose-built
// OpenStreetMap query service -- via Overpass QL. Unlike GDAL's local PBF
// file processing (see docs/DATA_SOURCES.md "OSM POIs"), it correctly
// returns real, stable IDs for way-mapped (building-outline) features, not
// just node-mapped (pushpin) ones.
type OverpassClient struct {
	httpClient *http.Client
}

// NewOverpassClient returns a client with a generous timeout -- Chicago-
// scale queries against the live API can take tens of seconds under load.
func NewOverpassClient() *OverpassClient {
	return &OverpassClient{httpClient: &http.Client{Timeout: 90 * time.Second}}
}

// OverpassLatLon is a bare lat/lon pair, used for a way's computed center
// point (requires "out center" in the query).
type OverpassLatLon struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// OverpassElement is one node or way from an Overpass QL result. Center and
// Geometry are populated only if the query asked for them ("out center" /
// "out geom" respectively) -- both are commonly used for ways, but serve
// different purposes: Center is a single representable point (e.g. for a
// POI), Geometry is the full ordered coordinate sequence (e.g. for a road
// or rail line).
type OverpassElement struct {
	Type     string            `json:"type"` // "node" or "way"
	ID       int64             `json:"id"`
	Lat      float64           `json:"lat"`      // node only
	Lon      float64           `json:"lon"`      // node only
	Center   *OverpassLatLon   `json:"center"`   // way only, requires "out center"
	Geometry []OverpassLatLon  `json:"geometry"` // way only, requires "out geom"
	Tags     map[string]string `json:"tags"`
}

// Point returns the element's representable point: its own coordinates for
// a node, or its computed center for a way. ok is false only if a way came
// back with no center, which "out center" should never produce -- a
// defensive guard, not an expected path.
func (e OverpassElement) Point() (lat, lon float64, ok bool) {
	if e.Type == "node" {
		return e.Lat, e.Lon, true
	}
	if e.Center != nil {
		return e.Center.Lat, e.Center.Lon, true
	}
	return 0, 0, false
}

type overpassResponse struct {
	Elements []OverpassElement `json:"elements"`
}

// Query runs a raw Overpass QL query and returns its elements. Transient
// failures (network errors, 429, 5xx) are retried with exponential
// backoff, mirroring SODAClient.GetJSON.
func (c *OverpassClient) Query(ctx context.Context, ql string) ([]OverpassElement, error) {
	var resp overpassResponse

	backoff := overpassInitialBackoff
	var lastErr error
	for attempt := 1; attempt <= overpassMaxAttempts; attempt++ {
		retryable, err := c.doQuery(ctx, ql, &resp)
		if err == nil {
			return resp.Elements, nil
		}
		lastErr = err

		if !retryable || attempt == overpassMaxAttempts {
			return nil, err
		}

		slog.Warn("Overpass query failed, retrying",
			slog.Int("attempt", attempt),
			slog.Int("max_attempts", overpassMaxAttempts),
			slog.String("error", err.Error()),
		)

		if err := overpassBackoffWait(ctx, backoff); err != nil {
			return nil, err
		}
		backoff *= 2
	}

	return nil, lastErr
}

func (c *OverpassClient) doQuery(ctx context.Context, ql string, dest any) (retryable bool, err error) {
	body := url.Values{"data": {ql}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, overpassBaseURL, strings.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("clients: build Overpass request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Overpass's usage policy asks consumers to identify their application
	// via User-Agent -- see docs/DATA_SOURCES.md "OSM POIs".
	req.Header.Set("User-Agent", "BlockFactor/1.0 (+https://github.com/KingsleyBawuah/block-factor-monorepo)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return true, fmt.Errorf("clients: Overpass request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response already consumed by the time this runs

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		reqErr := fmt.Errorf("clients: Overpass request returned status %d: %s", resp.StatusCode, respBody)
		return resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError, reqErr
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return false, fmt.Errorf("clients: decode Overpass response: %w", err)
	}

	return false, nil
}
