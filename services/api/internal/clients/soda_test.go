package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// withTestServer points sodaBaseURL at a temporary test server for the
// duration of the test and restores it afterward.
func withTestServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	original := sodaBaseURL
	sodaBaseURL = srv.URL
	t.Cleanup(func() { sodaBaseURL = original })
}

// withNoBackoffWait replaces sodaBackoffWait with a no-op for the duration
// of the test, so retry tests exercise the real retry/classification logic
// without actually waiting out sodaInitialBackoff between attempts. The
// real wait implementation is covered separately by
// TestSodaBackoffWait_RespectsContextCancellation.
func withNoBackoffWait(t *testing.T) {
	t.Helper()
	original := sodaBackoffWait
	sodaBackoffWait = func(ctx context.Context, _ time.Duration) error {
		return ctx.Err()
	}
	t.Cleanup(func() { sodaBackoffWait = original })
}

func TestGetJSON_DecodesResponseBody(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"name":"Chicago"}]`))
	})

	c := NewSODAClient("")
	var dest []struct {
		Name string `json:"name"`
	}
	if err := c.GetJSON(context.Background(), "fake", nil, &dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dest) != 1 || dest[0].Name != "Chicago" {
		t.Fatalf("unexpected decoded value: %+v", dest)
	}
}

func TestGetJSON_SetsAppTokenHeaderWhenProvided(t *testing.T) {
	var gotHeader string
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-App-Token")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})

	c := NewSODAClient("secret-token")
	var dest []any
	if err := c.GetJSON(context.Background(), "fake", nil, &dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHeader != "secret-token" {
		t.Fatalf("expected X-App-Token header %q, got %q", "secret-token", gotHeader)
	}
}

func TestGetJSON_OmitsAppTokenHeaderWhenEmpty(t *testing.T) {
	var sawHeader bool
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, sawHeader = r.Header["X-App-Token"]
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})

	c := NewSODAClient("")
	var dest []any
	if err := c.GetJSON(context.Background(), "fake", nil, &dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawHeader {
		t.Fatal("expected no X-App-Token header when appToken is empty")
	}
}

func TestGetJSON_RetriesOnServerErrorThenSucceeds(t *testing.T) {
	withNoBackoffWait(t)

	var calls int32
	withTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"message":"temporarily down"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})

	c := NewSODAClient("")
	var dest []any
	if err := c.GetJSON(context.Background(), "fake", nil, &dest); err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected exactly 3 calls (2 failures + 1 success), got %d", calls)
	}
}

func TestGetJSON_RetriesOnTooManyRequests(t *testing.T) {
	withNoBackoffWait(t)

	var calls int32
	withTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"rate limited"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})

	c := NewSODAClient("")
	var dest []any
	if err := c.GetJSON(context.Background(), "fake", nil, &dest); err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected exactly 2 calls, got %d", calls)
	}
}

func TestGetJSON_DoesNotRetryOnClientError(t *testing.T) {
	// Deliberately no withNoBackoffWait: a non-retryable error must return
	// before sodaBackoffWait is ever consulted, so this proves that too.
	var calls int32
	withTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"bad token"}`))
	})

	c := NewSODAClient("")
	var dest []any
	err := c.GetJSON(context.Background(), "fake", nil, &dest)

	if err == nil {
		t.Fatal("expected an error for a 403 response")
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 call (no retry on 4xx), got %d", calls)
	}
}

func TestGetJSON_ExhaustsRetriesAndReturnsLastError(t *testing.T) {
	withNoBackoffWait(t)

	var calls int32
	withTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"message":"still down"}`))
	})

	c := NewSODAClient("")
	var dest []any
	err := c.GetJSON(context.Background(), "fake", nil, &dest)

	if err == nil {
		t.Fatal("expected an error once retries are exhausted")
	}
	if calls != sodaMaxAttempts {
		t.Fatalf("expected exactly %d calls, got %d", sodaMaxAttempts, calls)
	}
}

func TestGetJSON_ReturnsErrorOnMalformedJSON(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	})

	c := NewSODAClient("")
	var dest []any
	if err := c.GetJSON(context.Background(), "fake", nil, &dest); err == nil {
		t.Fatal("expected a decode error for malformed JSON")
	}
}

// Sanity check that json.RawMessage-based dest types (used by every real
// dataset parser) round-trip correctly through GetJSON.
func TestGetJSON_DecodesRawMessageFields(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"the_geom":{"type":"Point","coordinates":[1,2]}}]`))
	})

	c := NewSODAClient("")
	var dest []struct {
		TheGeom json.RawMessage `json:"the_geom"`
	}
	if err := c.GetJSON(context.Background(), "fake", nil, &dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dest) != 1 || len(dest[0].TheGeom) == 0 {
		t.Fatalf("expected the_geom to be captured as raw JSON, got %+v", dest)
	}
}

// TestSodaBackoffWait_RespectsContextCancellation covers the real
// (production) sodaBackoffWait implementation directly, so the GetJSON-level
// retry tests above are free to mock it out without losing coverage of its
// actual behavior. Uses a short duration (not sodaInitialBackoff) so this
// stays fast on its own.
func TestSodaBackoffWait_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(10*time.Millisecond, cancel)

	start := time.Now()
	err := sodaBackoffWait(ctx, time.Hour) // would hang for an hour if cancellation didn't cut it short
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error when context is cancelled mid-wait")
	}
	if elapsed >= time.Second {
		t.Fatalf("expected cancellation to cut the wait short almost immediately, took %v", elapsed)
	}
}

func TestSodaBackoffWait_WaitsOutTheFullDuration(t *testing.T) {
	start := time.Now()
	if err := sodaBackoffWait(context.Background(), 20*time.Millisecond); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Fatalf("expected to wait at least 20ms, only waited %v", elapsed)
	}
}
