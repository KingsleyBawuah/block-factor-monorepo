package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// femaBaseURL is a var, not a const, so tests can point it at an
// httptest.Server instead of the real FEMA service.
var femaBaseURL = "https://hazards.fema.gov/arcgis/rest/services/public/NFHL/MapServer"

const (
	femaRetryBackoff = 2 * time.Second
	// femaMinPageSize is the floor for adaptive page shrinking -- see
	// QueryLayer's doc comment. Verified live that 25 records reliably
	// succeeds even in the densest window found in Chicago's data (199KB
	// response); a smaller floor was never needed in practice.
	femaMinPageSize = 5
)

// femaBackoffWait mirrors sodaBackoffWait/overpassBackoffWait -- overridable
// in tests so retry backoff doesn't cost real wall-clock time.
var femaBackoffWait = func(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// FEMAClient queries FEMA's National Flood Hazard Layer (NFHL) ArcGIS REST
// service (hazards.fema.gov) directly, rather than downloading a bulk
// shapefile/GDB extract -- see docs/DATA_SOURCES.md "Flood Zones".
type FEMAClient struct {
	httpClient *http.Client
}

// NewFEMAClient returns a client with a generous timeout -- NFHL polygon
// features are vertex-dense, and a single page can take a while to
// transfer.
func NewFEMAClient() *FEMAClient {
	return &FEMAClient{httpClient: &http.Client{Timeout: 60 * time.Second}}
}

// FEMAFeature is one feature from an NFHL layer query, decoded from
// GeoJSON. Geometry and Properties are left raw so callers can decode them
// into whatever shape matches the specific layer queried (different NFHL
// layers have different field sets).
type FEMAFeature struct {
	Type       string          `json:"type"`
	Geometry   json.RawMessage `json:"geometry"`
	Properties json.RawMessage `json:"properties"`
}

type femaFeatureCollection struct {
	Features []FEMAFeature `json:"features"`
}

// QueryLayer queries one NFHL MapServer layer (e.g. 28 for "Flood Hazard
// Zones") within a bounding box, paging through by however many features
// each window actually returns until every one of the layer's confirmed
// total count (via returnCountOnly) has been fetched.
//
// pageSize is a starting point, not a fixed size: some NFHL polygons are
// extremely large and complex (confirmed live -- a single window returned
// 15MB for just 50 features), which can trip a server-side response-size
// ceiling independent of the row-count limit. This is deterministic, not
// transient -- the same offset fails at a larger record count and
// succeeds at a smaller one, every time -- so on a retryable failure this
// halves the attempted window size and retries the same offset (down to
// femaMinPageSize) instead of blindly retrying an identical request that
// would just fail the same way again.
func (c *FEMAClient) QueryLayer(ctx context.Context, layerID int, bboxSouth, bboxWest, bboxNorth, bboxEast float64, outFields string, pageSize int) ([]FEMAFeature, error) {
	total, err := c.Count(ctx, layerID, bboxSouth, bboxWest, bboxNorth, bboxEast)
	if err != nil {
		return nil, err
	}

	all := make([]FEMAFeature, 0, total)
	offset := 0
	for offset < total {
		page, err := c.queryWindowAdaptive(ctx, layerID, bboxSouth, bboxWest, bboxNorth, bboxEast, outFields, offset, pageSize)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			return nil, fmt.Errorf("clients: FEMA query returned no features at offset %d (expected more, total is %d)", offset, total)
		}
		all = append(all, page...)
		offset += len(page)
	}
	return all, nil
}

// queryWindowAdaptive fetches one window starting at offset, shrinking
// attemptSize on a retryable failure until it succeeds or hits
// femaMinPageSize.
func (c *FEMAClient) queryWindowAdaptive(ctx context.Context, layerID int, south, west, north, east float64, outFields string, offset, attemptSize int) ([]FEMAFeature, error) {
	for {
		var fc femaFeatureCollection
		retryable, err := c.doQuery(ctx, layerID, south, west, north, east, outFields, attemptSize, offset, &fc)
		if err == nil {
			return fc.Features, nil
		}
		if !retryable {
			return nil, err
		}
		if attemptSize <= femaMinPageSize {
			return nil, fmt.Errorf("clients: FEMA query still failing at minimum window size %d, offset %d: %w", femaMinPageSize, offset, err)
		}

		attemptSize /= 2
		if attemptSize < femaMinPageSize {
			attemptSize = femaMinPageSize
		}
		slog.Warn("FEMA page request failed, shrinking window and retrying",
			slog.Int("offset", offset),
			slog.Int("new_attempt_size", attemptSize),
			slog.String("error", err.Error()),
		)
		if err := femaBackoffWait(ctx, femaRetryBackoff); err != nil {
			return nil, err
		}
	}
}

// Count returns the total number of features matching the bounding box on
// the given layer, via returnCountOnly -- used to know exactly when
// pagination is complete rather than inferring it from a short page (which
// adaptive window sizing makes ambiguous).
func (c *FEMAClient) Count(ctx context.Context, layerID int, south, west, north, east float64) (int, error) {
	endpoint := fmt.Sprintf("%s/%d/query", femaBaseURL, layerID)
	params := url.Values{
		"geometry":        {fmt.Sprintf("%f,%f,%f,%f", west, south, east, north)},
		"geometryType":    {"esriGeometryEnvelope"},
		"inSR":            {"4326"},
		"spatialRel":      {"esriSpatialRelIntersects"},
		"returnCountOnly": {"true"},
		"f":               {"json"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return 0, fmt.Errorf("clients: build FEMA count request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("clients: FEMA count request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response already consumed by the time this runs

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return 0, fmt.Errorf("clients: FEMA count request returned status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("clients: decode FEMA count response: %w", err)
	}
	return result.Count, nil
}

func (c *FEMAClient) doQuery(ctx context.Context, layerID int, south, west, north, east float64, outFields string, pageSize, offset int, dest any) (retryable bool, err error) {
	endpoint := fmt.Sprintf("%s/%d/query", femaBaseURL, layerID)
	params := url.Values{
		"geometry":          {fmt.Sprintf("%f,%f,%f,%f", west, south, east, north)},
		"geometryType":      {"esriGeometryEnvelope"},
		"inSR":              {"4326"},
		"spatialRel":        {"esriSpatialRelIntersects"},
		"outFields":         {outFields},
		"returnGeometry":    {"true"},
		"f":                 {"geojson"},
		"resultRecordCount": {strconv.Itoa(pageSize)},
		"resultOffset":      {strconv.Itoa(offset)},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return false, fmt.Errorf("clients: build FEMA request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return true, fmt.Errorf("clients: FEMA request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response already consumed by the time this runs

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		reqErr := fmt.Errorf("clients: FEMA request (layer %d, offset %d) returned status %d: %s", layerID, offset, resp.StatusCode, body)
		return resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError, reqErr
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return false, fmt.Errorf("clients: decode FEMA response: %w", err)
	}

	return false, nil
}
