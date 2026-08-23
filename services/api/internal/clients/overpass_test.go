package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func withOverpassTestServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	original := overpassBaseURL
	overpassBaseURL = srv.URL
	t.Cleanup(func() { overpassBaseURL = original })
}

func withNoOverpassBackoffWait(t *testing.T) {
	t.Helper()
	original := overpassBackoffWait
	overpassBackoffWait = func(ctx context.Context, _ time.Duration) error {
		return ctx.Err()
	}
	t.Cleanup(func() { overpassBackoffWait = original })
}

func TestOverpassQuery_DecodesElements(t *testing.T) {
	withOverpassTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"elements": [
				{"type":"node","id":123,"lat":41.85,"lon":-87.65,"tags":{"shop":"supermarket","name":"Test Store"}},
				{"type":"way","id":456,"center":{"lat":41.86,"lon":-87.66},"tags":{"amenity":"pharmacy"}}
			]
		}`))
	})

	c := NewOverpassClient()
	elements, err := c.Query(context.Background(), "fake query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(elements))
	}

	lat, lon, ok := elements[0].Point()
	if !ok || lat != 41.85 || lon != -87.65 {
		t.Errorf("node Point() = (%v, %v, %v), want (41.85, -87.65, true)", lat, lon, ok)
	}
	lat, lon, ok = elements[1].Point()
	if !ok || lat != 41.86 || lon != -87.66 {
		t.Errorf("way Point() = (%v, %v, %v), want (41.86, -87.66, true)", lat, lon, ok)
	}
}

func TestOverpassElement_WayWithNoCenterHasNoPoint(t *testing.T) {
	el := OverpassElement{Type: "way", ID: 1}
	_, _, ok := el.Point()
	if ok {
		t.Fatal("expected ok=false for a way with no center")
	}
}

func TestOverpassQuery_SetsUserAgentHeader(t *testing.T) {
	var gotUA string
	withOverpassTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"elements":[]}`))
	})

	c := NewOverpassClient()
	if _, err := c.Query(context.Background(), "fake query"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotUA == "" || gotUA == "Go-http-client/1.1" {
		t.Fatalf("expected a real identifying User-Agent, got %q", gotUA)
	}
}

func TestOverpassQuery_RetriesOnServerErrorThenSucceeds(t *testing.T) {
	withNoOverpassBackoffWait(t)

	var calls int32
	withOverpassTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`error`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"elements":[]}`))
	})

	c := NewOverpassClient()
	if _, err := c.Query(context.Background(), "fake query"); err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected exactly 3 calls, got %d", calls)
	}
}

func TestOverpassQuery_DoesNotRetryOnClientError(t *testing.T) {
	var calls int32
	withOverpassTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`bad query syntax`))
	})

	c := NewOverpassClient()
	_, err := c.Query(context.Background(), "fake query")
	if err == nil {
		t.Fatal("expected an error for a 400 response")
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 call (no retry on 4xx), got %d", calls)
	}
}
