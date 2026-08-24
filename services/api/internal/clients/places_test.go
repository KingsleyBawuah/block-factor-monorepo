package clients

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func withNoPlacesBackoff(t *testing.T) {
	t.Helper()
	orig := placesBackoffWait
	placesBackoffWait = func(_ context.Context, _ time.Duration) error { return nil }
	t.Cleanup(func() { placesBackoffWait = orig })
}

func TestPlacesClient_SearchNearby_Success(t *testing.T) {
	var capturedBody placesSearchNearbyRequest
	var capturedAPIKey, capturedFieldMask string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAPIKey = r.Header.Get("X-Goog-Api-Key")
		capturedFieldMask = r.Header.Get("X-Goog-FieldMask")
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(placesSearchNearbyResponse{
			Places: []placesRawPlace{
				{
					ID:          "place-1",
					DisplayName: placesLocalizedText{Text: "Sip Coffee House"},
					PrimaryType: "cafe",
					Types:       []string{"cafe", "food"},
					Location:    placesLatLng{Latitude: 41.9, Longitude: -87.65},
				},
				{
					ID:          "place-2",
					DisplayName: placesLocalizedText{Text: "Lincoln Park Fitness"},
					PrimaryType: "gym",
					Types:       []string{"gym"},
					Location:    placesLatLng{Latitude: 41.901, Longitude: -87.651},
				},
			},
		})
	}))
	defer server.Close()

	client := NewPlacesClient("test-api-key", slog.New(slog.DiscardHandler))
	client.baseURL = server.URL
	client.limiter = rate.NewLimiter(rate.Inf, 1)

	places, err := client.SearchNearby(context.Background(), 41.8998, -87.6505, 804.672, []string{"cafe", "gym", "fitness_center"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedAPIKey != "test-api-key" {
		t.Errorf("expected X-Goog-Api-Key %q, got %q", "test-api-key", capturedAPIKey)
	}
	if capturedFieldMask != placesFieldMask {
		t.Errorf("expected X-Goog-FieldMask %q, got %q", placesFieldMask, capturedFieldMask)
	}
	if capturedBody.MaxResultCount != placesMaxResultCount {
		t.Errorf("expected maxResultCount %d, got %d", placesMaxResultCount, capturedBody.MaxResultCount)
	}
	if capturedBody.LocationRestriction.Circle.Radius != 804.672 {
		t.Errorf("expected radius 804.672, got %v", capturedBody.LocationRestriction.Circle.Radius)
	}
	if capturedBody.LocationRestriction.Circle.Center.Latitude != 41.8998 || capturedBody.LocationRestriction.Circle.Center.Longitude != -87.6505 {
		t.Errorf("unexpected center: %+v", capturedBody.LocationRestriction.Circle.Center)
	}

	if len(places) != 2 {
		t.Fatalf("expected 2 places, got %d", len(places))
	}
	if places[0].ID != "place-1" || places[0].Name != "Sip Coffee House" || places[0].PrimaryType != "cafe" || places[0].Lat != 41.9 || places[0].Lon != -87.65 {
		t.Errorf("unexpected places[0]: %+v", places[0])
	}
	if places[1].ID != "place-2" || places[1].Name != "Lincoln Park Fitness" || places[1].PrimaryType != "gym" {
		t.Errorf("unexpected places[1]: %+v", places[1])
	}
}

func TestPlacesClient_SearchNearby_MissingAPIKey(t *testing.T) {
	client := NewPlacesClient("", slog.New(slog.DiscardHandler))
	_, err := client.SearchNearby(context.Background(), 41.8998, -87.6505, 804.672, []string{"cafe"})
	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
}

func TestPlacesClient_SearchNearby_EmptyIncludedTypes(t *testing.T) {
	client := NewPlacesClient("test-api-key", slog.New(slog.DiscardHandler))
	_, err := client.SearchNearby(context.Background(), 41.8998, -87.6505, 804.672, nil)
	if err == nil {
		t.Fatal("expected error for empty includedTypes, got nil")
	}
}

func TestPlacesClient_SearchNearby_RetriesTransientFailureThenSucceeds(t *testing.T) {
	withNoPlacesBackoff(t)

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"temporarily unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(placesSearchNearbyResponse{Places: nil})
	}))
	defer server.Close()

	client := NewPlacesClient("test-api-key", slog.New(slog.DiscardHandler))
	client.baseURL = server.URL
	client.limiter = rate.NewLimiter(rate.Inf, 1)

	places, err := client.SearchNearby(context.Background(), 41.8998, -87.6505, 804.672, []string{"cafe"})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if len(places) != 0 {
		t.Errorf("expected 0 places, got %d", len(places))
	}
}

func TestPlacesClient_SearchNearby_TooManyRequestsRetry(t *testing.T) {
	withNoPlacesBackoff(t)

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`rate limited`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(placesSearchNearbyResponse{Places: nil})
	}))
	defer server.Close()

	client := NewPlacesClient("test-api-key", slog.New(slog.DiscardHandler))
	client.baseURL = server.URL
	client.limiter = rate.NewLimiter(rate.Inf, 1)

	_, err := client.SearchNearby(context.Background(), 41.8998, -87.6505, 804.672, []string{"cafe"})
	if err != nil {
		t.Fatalf("expected success after 429 retry, got: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestPlacesClient_SearchNearby_NonRetryableClientError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`bad request`))
	}))
	defer server.Close()

	client := NewPlacesClient("test-api-key", slog.New(slog.DiscardHandler))
	client.baseURL = server.URL
	client.limiter = rate.NewLimiter(rate.Inf, 1)

	_, err := client.SearchNearby(context.Background(), 41.8998, -87.6505, 804.672, []string{"cafe"})
	if err == nil {
		t.Fatal("expected error on 400 Bad Request, got nil")
	}
	if attempts != 1 {
		t.Errorf("expected non-retryable 400 to execute exactly 1 attempt, got %d", attempts)
	}
}

func TestPlacesClient_SearchNearby_ContextCancelledDuringBackoffWait(t *testing.T) {
	// Deliberately does NOT call withNoPlacesBackoff -- this exercises the
	// real placesBackoffWait's ctx.Done() branch, which every other retry
	// test bypasses.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"temporarily unavailable"}`))
	}))
	defer server.Close()

	client := NewPlacesClient("test-api-key", slog.New(slog.DiscardHandler))
	client.baseURL = server.URL
	client.limiter = rate.NewLimiter(rate.Inf, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := client.SearchNearby(ctx, 41.8998, -87.6505, 804.672, []string{"cafe"})
	if err == nil {
		t.Fatal("expected error once the context is cancelled mid-backoff, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
}
