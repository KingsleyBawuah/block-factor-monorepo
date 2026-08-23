package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// sodaBaseURL is a var, not a const, so tests can point it at an
// httptest.Server instead of the real Chicago Data Portal.
var sodaBaseURL = "https://data.cityofchicago.org/resource"

const (
	sodaMaxAttempts    = 3
	sodaInitialBackoff = 1 * time.Second
)

// sodaBackoffWait is overridable in tests so retry backoff doesn't cost
// real wall-clock time -- production uses a real timer that still respects
// context cancellation. See TestSodaBackoffWait_RespectsContextCancellation
// for direct coverage of this default implementation, and withNoBackoffWait
// in soda_test.go for how the GetJSON-level retry tests skip the wait
// entirely.
var sodaBackoffWait = func(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// SODAClient calls the Chicago Data Portal's Socrata Open Data API (SODA).
type SODAClient struct {
	httpClient *http.Client
	appToken   string
}

// NewSODAClient returns a client authenticated with appToken via the
// X-App-Token header (SODA's preferred auth mechanism, per Socrata's own
// docs). An empty appToken still works — it just falls back to Socrata's
// much lower unauthenticated, IP-based rate limit.
func NewSODAClient(appToken string) *SODAClient {
	return &SODAClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		appToken:   appToken,
	}
}

// GetJSON fetches a dataset resource (identified by its SODA resource ID,
// e.g. "qqq8-j68g") with optional SoQL query parameters (e.g. "$where",
// "$limit") and decodes the JSON response into dest.
//
// Transient failures — network errors, 429 rate limiting, 5xx server
// errors — are retried with exponential backoff (up to sodaMaxAttempts
// total attempts). Other errors (bad token, bad query, not found) fail
// immediately, since retrying them wastes time without fixing anything.
func (c *SODAClient) GetJSON(ctx context.Context, resourceID string, params url.Values, dest any) error {
	endpoint := fmt.Sprintf("%s/%s.json", sodaBaseURL, resourceID)
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	backoff := sodaInitialBackoff
	var lastErr error
	for attempt := 1; attempt <= sodaMaxAttempts; attempt++ {
		retryable, err := c.fetchJSON(ctx, endpoint, resourceID, dest)
		if err == nil {
			return nil
		}
		lastErr = err

		if !retryable || attempt == sodaMaxAttempts {
			return err
		}

		slog.Warn("SODA request failed, retrying",
			slog.String("resource_id", resourceID),
			slog.Int("attempt", attempt),
			slog.Int("max_attempts", sodaMaxAttempts),
			slog.String("error", err.Error()),
		)

		if err := sodaBackoffWait(ctx, backoff); err != nil {
			return err
		}
		backoff *= 2
	}

	return lastErr
}

// fetchJSON makes one attempt. The retryable return indicates whether a
// failure is transient and worth retrying: network-level errors and
// 429/5xx responses are; other 4xx responses are not, since the request
// itself is what's wrong.
func (c *SODAClient) fetchJSON(ctx context.Context, endpoint, resourceID string, dest any) (retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, fmt.Errorf("clients: build SODA request: %w", err)
	}
	if c.appToken != "" {
		req.Header.Set("X-App-Token", c.appToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return true, fmt.Errorf("clients: SODA request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response already consumed by the time this runs

	if resp.StatusCode != http.StatusOK {
		// Socrata returns a JSON error body ({"message": ..., "errorCode": ...})
		// that's invaluable for debugging a bad/missing token (403) or rate
		// limiting (429) — worth the extra read over just the status code.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		reqErr := fmt.Errorf("clients: SODA request to %s returned status %d: %s", resourceID, resp.StatusCode, body)
		return resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError, reqErr
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return false, fmt.Errorf("clients: decode SODA response: %w", err)
	}

	return false, nil
}
