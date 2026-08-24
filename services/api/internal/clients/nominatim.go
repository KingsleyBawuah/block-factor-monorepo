package clients

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// nominatimBaseURL is overridable in tests via httptest.Server.
var nominatimBaseURL = "https://nominatim.openstreetmap.org/search"

const (
	nominatimMaxAttempts    = 3
	nominatimInitialBackoff = 1 * time.Second
	// Default Chicago bounding box (minLon, maxLat, maxLon, minLat for viewbox)
	// Covering the Chicago metropolitan area to bias search results locally.
	chicagoViewbox = "-87.9401,42.0231,-87.5240,41.6443"
)

// nominatimBackoffWait is overridable in tests so retry backoff doesn't cost real wall-clock time.
var nominatimBackoffWait = func(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// NominatimRawResult mirrors the JSON structure returned by Nominatim API.
type NominatimRawResult struct {
	PlaceID     int64    `json:"place_id"`
	OsmType     string   `json:"osm_type"`
	OsmID       int64    `json:"osm_id"`
	Lat         string   `json:"lat"`
	Lon         string   `json:"lon"`
	DisplayName string   `json:"display_name"`
	BoundingBox []string `json:"boundingbox"`
	Importance  float64  `json:"importance"`
}

// GeocodeCandidate represents a normalized geocoded address match.
type GeocodeCandidate struct {
	FormattedAddress string
	Lat              float64
	Lon              float64
	Rank             int
}

// NominatimClient interacts with the OpenStreetMap Nominatim geocoding API.
type NominatimClient struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	limiter    *rate.Limiter
	logger     *slog.Logger
}

// NewNominatimClient creates a NominatimClient enforcing the 1 req/sec rate limit.
func NewNominatimClient(userAgent string, logger *slog.Logger) *NominatimClient {
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "BlockFactor/0.1 (contact: engineering@blockfactor.io)"
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &NominatimClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    nominatimBaseURL,
		userAgent:  userAgent,
		// Enforce strict 1 req/sec per Nominatim Usage Policy
		limiter: rate.NewLimiter(rate.Every(1*time.Second), 1),
		logger:  logger,
	}
}

// Geocode queries the Nominatim API for the given query string and returns ranked candidates.
func (c *NominatimClient) Geocode(ctx context.Context, query string) ([]GeocodeCandidate, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, errors.New("clients: geocode query cannot be empty")
	}

	params := url.Values{}
	params.Set("q", trimmed)
	params.Set("format", "jsonv2")
	params.Set("addressdetails", "1")
	params.Set("limit", "5")
	params.Set("countrycodes", "us")
	params.Set("viewbox", chicagoViewbox)

	reqURL := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())

	var lastErr error
	backoff := nominatimInitialBackoff

	for attempt := 1; attempt <= nominatimMaxAttempts; attempt++ {
		retryable, candidates, err := c.fetchCandidates(ctx, reqURL)
		if err == nil {
			return candidates, nil
		}

		lastErr = err
		if !retryable || attempt == nominatimMaxAttempts {
			return nil, lastErr
		}

		c.logger.Warn("Nominatim request failed, retrying",
			slog.Int("attempt", attempt),
			slog.Int("max_attempts", nominatimMaxAttempts),
			slog.String("error", err.Error()),
		)

		if err := nominatimBackoffWait(ctx, backoff); err != nil {
			return nil, err
		}
		backoff *= 2
	}

	return nil, lastErr
}

func (c *NominatimClient) fetchCandidates(ctx context.Context, reqURL string) (retryable bool, candidates []GeocodeCandidate, err error) {
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return false, nil, fmt.Errorf("clients: rate limiter wait failed: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, nil, fmt.Errorf("clients: build nominatim request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return true, nil, fmt.Errorf("clients: execute nominatim request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return true, nil, fmt.Errorf("clients: Nominatim returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return false, nil, fmt.Errorf("clients: Nominatim non-retryable status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var rawResults []NominatimRawResult
	if err := json.NewDecoder(resp.Body).Decode(&rawResults); err != nil {
		return false, nil, fmt.Errorf("clients: decode nominatim response: %w", err)
	}

	candidates = make([]GeocodeCandidate, 0, len(rawResults))
	for i, raw := range rawResults {
		lat, err := strconv.ParseFloat(raw.Lat, 64)
		if err != nil {
			c.logger.Warn("skipping candidate with invalid latitude", slog.String("lat", raw.Lat))
			continue
		}
		lon, err := strconv.ParseFloat(raw.Lon, 64)
		if err != nil {
			c.logger.Warn("skipping candidate with invalid longitude", slog.String("lon", raw.Lon))
			continue
		}

		candidates = append(candidates, GeocodeCandidate{
			FormattedAddress: raw.DisplayName,
			Lat:              lat,
			Lon:              lon,
			Rank:             i,
		})
	}

	return false, candidates, nil
}
