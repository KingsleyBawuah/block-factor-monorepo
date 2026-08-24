package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// placesBaseURL is overridable in tests via httptest.Server.
var placesBaseURL = "https://places.googleapis.com/v1/places:searchNearby"

const (
	placesMaxAttempts    = 3
	placesInitialBackoff = 1 * time.Second

	// placesMaxResultCount is the Places API's own per-request ceiling.
	placesMaxResultCount = 20

	// placesFieldMask limits the response to exactly the fields BlockFactor
	// scores against -- every additional field Google returns is billed at
	// a higher SKU tier, so the mask itself is a cost control.
	placesFieldMask = "places.id,places.displayName,places.primaryType,places.types,places.location"
)

// placesBackoffWait mirrors nominatimBackoffWait/overpassBackoffWait --
// overridable in tests so retry backoff doesn't cost real wall-clock time.
var placesBackoffWait = func(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// Place is a normalized Google Places (New) Nearby Search result.
type Place struct {
	ID          string
	Name        string
	PrimaryType string
	Types       []string
	Lat         float64
	Lon         float64
}

// PlacesClient queries the Google Places API (New) Nearby Search endpoint
// (`places:searchNearby`). Results are not cached here -- the caller (the
// S_third_place grid-cache fill in internal/scoring) owns caching, since
// caching is about *which coordinate bucket* to key on, a scoring-layer
// concern this client has no opinion about.
type PlacesClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	limiter    *rate.Limiter
	logger     *slog.Logger
}

// NewPlacesClient creates a PlacesClient. The rate limit is a self-imposed
// outbound pacing guard (cost/quota protection), not a documented Google
// requirement the way Nominatim's 1 req/sec is a policy mandate.
func NewPlacesClient(apiKey string, logger *slog.Logger) *PlacesClient {
	if logger == nil {
		logger = slog.Default()
	}

	return &PlacesClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    placesBaseURL,
		apiKey:     apiKey,
		limiter:    rate.NewLimiter(rate.Limit(10), 1),
		logger:     logger,
	}
}

type placesSearchNearbyRequest struct {
	IncludedTypes       []string                  `json:"includedTypes"`
	MaxResultCount      int                       `json:"maxResultCount"`
	LocationRestriction placesLocationRestriction `json:"locationRestriction"`
}

type placesLocationRestriction struct {
	Circle placesCircle `json:"circle"`
}

type placesCircle struct {
	Center placesLatLng `json:"center"`
	Radius float64      `json:"radius"`
}

type placesLatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type placesLocalizedText struct {
	Text string `json:"text"`
}

type placesRawPlace struct {
	ID          string              `json:"id"`
	DisplayName placesLocalizedText `json:"displayName"`
	PrimaryType string              `json:"primaryType"`
	Types       []string            `json:"types"`
	Location    placesLatLng        `json:"location"`
}

type placesSearchNearbyResponse struct {
	Places []placesRawPlace `json:"places"`
}

// SearchNearby returns places matching any of includedTypes within
// radiusMeters of (lat, lon). Transient failures (network errors, 429, 5xx)
// are retried with exponential backoff, mirroring OverpassClient.Query.
func (c *PlacesClient) SearchNearby(ctx context.Context, lat, lon, radiusMeters float64, includedTypes []string) ([]Place, error) {
	if c.apiKey == "" {
		return nil, errors.New("clients: places API key not configured")
	}
	if len(includedTypes) == 0 {
		return nil, errors.New("clients: places search requires at least one included type")
	}

	reqBody := placesSearchNearbyRequest{
		IncludedTypes:  includedTypes,
		MaxResultCount: placesMaxResultCount,
		LocationRestriction: placesLocationRestriction{
			Circle: placesCircle{
				Center: placesLatLng{Latitude: lat, Longitude: lon},
				Radius: radiusMeters,
			},
		},
	}

	var lastErr error
	backoff := placesInitialBackoff
	for attempt := 1; attempt <= placesMaxAttempts; attempt++ {
		retryable, places, err := c.doSearch(ctx, reqBody)
		if err == nil {
			return places, nil
		}
		lastErr = err

		if !retryable || attempt == placesMaxAttempts {
			return nil, lastErr
		}

		c.logger.Warn("Places nearby search failed, retrying",
			slog.Int("attempt", attempt),
			slog.Int("max_attempts", placesMaxAttempts),
			slog.String("error", err.Error()),
		)

		if err := placesBackoffWait(ctx, backoff); err != nil {
			return nil, err
		}
		backoff *= 2
	}

	return nil, lastErr
}

func (c *PlacesClient) doSearch(ctx context.Context, reqBody placesSearchNearbyRequest) (retryable bool, places []Place, err error) {
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return false, nil, fmt.Errorf("clients: rate limiter wait failed: %w", err)
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return false, nil, fmt.Errorf("clients: encode places request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return false, nil, fmt.Errorf("clients: build places request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", c.apiKey)
	req.Header.Set("X-Goog-FieldMask", placesFieldMask)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return true, nil, fmt.Errorf("clients: execute places request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response already consumed by the time this runs

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return true, nil, fmt.Errorf("clients: places request returned status %d: %s", resp.StatusCode, respBody)
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return false, nil, fmt.Errorf("clients: places request returned non-retryable status %d: %s", resp.StatusCode, respBody)
	}

	var raw placesSearchNearbyResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return false, nil, fmt.Errorf("clients: decode places response: %w", err)
	}

	places = make([]Place, 0, len(raw.Places))
	for _, p := range raw.Places {
		places = append(places, Place{
			ID:          p.ID,
			Name:        p.DisplayName.Text,
			PrimaryType: p.PrimaryType,
			Types:       p.Types,
			Lat:         p.Location.Latitude,
			Lon:         p.Location.Longitude,
		})
	}

	return false, places, nil
}
