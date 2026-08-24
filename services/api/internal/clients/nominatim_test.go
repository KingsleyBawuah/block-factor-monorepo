package clients

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func withNoNominatimBackoff(t *testing.T) {
	t.Helper()
	orig := nominatimBackoffWait
	nominatimBackoffWait = func(_ context.Context, _ time.Duration) error { return nil }
	t.Cleanup(func() { nominatimBackoffWait = orig })
}

func TestNominatimClient_Geocode_Success(t *testing.T) {
	mockResults := []NominatimRawResult{
		{
			PlaceID:     12345,
			DisplayName: "233 S Wacker Dr, Loop, Chicago, IL 60606",
			Lat:         "41.878876",
			Lon:         "-87.635915",
			Importance:  0.85,
		},
		{
			PlaceID:     67890,
			DisplayName: "233 N Michigan Ave, Loop, Chicago, IL 60601",
			Lat:         "41.886450",
			Lon:         "-87.624730",
			Importance:  0.72,
		},
	}

	var capturedUserAgent string
	var capturedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserAgent = r.Header.Get("User-Agent")
		capturedQuery = r.URL.Query().Get("q")
		if r.URL.Query().Get("format") != "jsonv2" {
			t.Errorf("expected format=jsonv2, got %q", r.URL.Query().Get("format"))
		}
		if r.URL.Query().Get("viewbox") != chicagoViewbox {
			t.Errorf("expected viewbox=%s, got %q", chicagoViewbox, r.URL.Query().Get("viewbox"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResults)
	}))
	defer server.Close()

	client := NewNominatimClient("BlockFactorTest/1.0", slog.New(slog.DiscardHandler))
	client.baseURL = server.URL
	// Fast rate limiter for tests
	client.limiter = rate.NewLimiter(rate.Inf, 1)

	ctx := context.Background()
	candidates, err := client.Geocode(ctx, "233 S Wacker Dr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedUserAgent != "BlockFactorTest/1.0" {
		t.Errorf("expected User-Agent %q, got %q", "BlockFactorTest/1.0", capturedUserAgent)
	}
	if capturedQuery != "233 S Wacker Dr" {
		t.Errorf("expected query %q, got %q", "233 S Wacker Dr", capturedQuery)
	}

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}

	if candidates[0].Rank != 0 || candidates[0].FormattedAddress != mockResults[0].DisplayName || candidates[0].Lat != 41.878876 || candidates[0].Lon != -87.635915 {
		t.Errorf("unexpected candidate[0]: %+v", candidates[0])
	}
	if candidates[1].Rank != 1 || candidates[1].FormattedAddress != mockResults[1].DisplayName || candidates[1].Lat != 41.886450 || candidates[1].Lon != -87.624730 {
		t.Errorf("unexpected candidate[1]: %+v", candidates[1])
	}
}

func TestNominatimClient_Geocode_EmptyQuery(t *testing.T) {
	client := NewNominatimClient("", slog.New(slog.DiscardHandler))
	_, err := client.Geocode(context.Background(), "   ")
	if err == nil {
		t.Fatal("expected error for empty query, got nil")
	}
}

func TestNominatimClient_Geocode_RetriesTransientFailureThenSucceeds(t *testing.T) {
	withNoNominatimBackoff(t)

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"temporarily unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]NominatimRawResult{
			{
				PlaceID:     1,
				DisplayName: "Chicago, IL",
				Lat:         "41.8781",
				Lon:         "-87.6298",
			},
		})
	}))
	defer server.Close()

	client := NewNominatimClient("", slog.New(slog.DiscardHandler))
	client.baseURL = server.URL
	client.limiter = rate.NewLimiter(rate.Inf, 1)

	candidates, err := client.Geocode(context.Background(), "Chicago")
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
}

func TestNominatimClient_Geocode_TooManyRequestsRetry(t *testing.T) {
	withNoNominatimBackoff(t)

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`rate limited`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]NominatimRawResult{
			{
				PlaceID:     1,
				DisplayName: "Chicago, IL",
				Lat:         "41.8781",
				Lon:         "-87.6298",
			},
		})
	}))
	defer server.Close()

	client := NewNominatimClient("", slog.New(slog.DiscardHandler))
	client.baseURL = server.URL
	client.limiter = rate.NewLimiter(rate.Inf, 1)

	candidates, err := client.Geocode(context.Background(), "Chicago")
	if err != nil {
		t.Fatalf("expected success after 429 retry, got: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
}

func TestNominatimClient_Geocode_NonRetryableClientError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`bad request`))
	}))
	defer server.Close()

	client := NewNominatimClient("", slog.New(slog.DiscardHandler))
	client.baseURL = server.URL
	client.limiter = rate.NewLimiter(rate.Inf, 1)

	_, err := client.Geocode(context.Background(), "invalid query")
	if err == nil {
		t.Fatal("expected error on 400 Bad Request, got nil")
	}
	if attempts != 1 {
		t.Errorf("expected non-retryable 400 to execute exactly 1 attempt, got %d", attempts)
	}
}

func TestNominatimClient_Geocode_InvalidCoordinateStringsSkipped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]NominatimRawResult{
			{
				PlaceID:     1,
				DisplayName: "Bad Lat",
				Lat:         "not-a-number",
				Lon:         "-87.6298",
			},
			{
				PlaceID:     2,
				DisplayName: "Valid Coordinate",
				Lat:         "41.8781",
				Lon:         "-87.6298",
			},
		})
	}))
	defer server.Close()

	client := NewNominatimClient("", slog.New(slog.DiscardHandler))
	client.baseURL = server.URL
	client.limiter = rate.NewLimiter(rate.Inf, 1)

	candidates, err := client.Geocode(context.Background(), "Chicago")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 valid candidate after skipping bad lat, got %d", len(candidates))
	}
	if candidates[0].FormattedAddress != "Valid Coordinate" {
		t.Errorf("expected candidate 'Valid Coordinate', got %q", candidates[0].FormattedAddress)
	}
}
