package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const sodaBaseURL = "https://data.cityofchicago.org/resource"

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
func (c *SODAClient) GetJSON(ctx context.Context, resourceID string, params url.Values, dest any) error {
	endpoint := fmt.Sprintf("%s/%s.json", sodaBaseURL, resourceID)
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("clients: build SODA request: %w", err)
	}
	if c.appToken != "" {
		req.Header.Set("X-App-Token", c.appToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("clients: SODA request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response already consumed by the time this runs

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("clients: SODA request to %s returned status %d", resourceID, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("clients: decode SODA response: %w", err)
	}

	return nil
}
