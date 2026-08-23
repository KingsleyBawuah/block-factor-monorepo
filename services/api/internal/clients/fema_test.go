package clients

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func withFEMATestServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	original := femaBaseURL
	femaBaseURL = srv.URL
	t.Cleanup(func() { femaBaseURL = original })
}

func withNoFEMABackoffWait(t *testing.T) {
	t.Helper()
	original := femaBackoffWait
	femaBackoffWait = func(ctx context.Context, _ time.Duration) error {
		return ctx.Err()
	}
	t.Cleanup(func() { femaBackoffWait = original })
}

// featureCollectionJSON builds a minimal valid GeoJSON FeatureCollection
// with n trivial point features, enough to exercise pagination logic
// without needing real flood-zone geometry.
func featureCollectionJSON(n int) string {
	features := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			features += ","
		}
		features += `{"type":"Feature","geometry":{"type":"Point","coordinates":[0,0]},"properties":{}}`
	}
	return fmt.Sprintf(`{"type":"FeatureCollection","features":[%s]}`, features)
}

func TestFEMACount(t *testing.T) {
	withFEMATestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("returnCountOnly") != "true" {
			t.Errorf("expected returnCountOnly=true, got query %q", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"count":2541}`))
	})

	c := NewFEMAClient()
	count, err := c.Count(context.Background(), 28, 41.6, -87.9, 42.0, -87.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2541 {
		t.Fatalf("Count() = %d, want 2541", count)
	}
}

func TestFEMACount_RetriesTransientFailureThenSucceeds(t *testing.T) {
	withNoFEMABackoffWait(t)

	var calls int32
	withFEMATestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("temporarily unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"count":2541}`))
	})

	c := NewFEMAClient()
	count, err := c.Count(context.Background(), 28, 41.6, -87.9, 42.0, -87.5)
	if err != nil {
		t.Fatalf("expected success after one retry, got error: %v", err)
	}
	if count != 2541 {
		t.Fatalf("Count() = %d, want 2541", count)
	}
	if calls != 2 {
		t.Fatalf("expected exactly 2 attempts, got %d", calls)
	}
}

func TestFEMACount_GivesUpAfterMaxRetries(t *testing.T) {
	withNoFEMABackoffWait(t)

	var calls int32
	withFEMATestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("always failing"))
	})

	c := NewFEMAClient()
	_, err := c.Count(context.Background(), 28, 41.6, -87.9, 42.0, -87.5)
	if err == nil {
		t.Fatal("expected an error when Count always fails")
	}
	if calls != femaMaxRetryAttempts {
		t.Fatalf("expected exactly %d attempts, got %d", femaMaxRetryAttempts, calls)
	}
}

func TestFEMACount_DoesNotRetryNonRetryableError(t *testing.T) {
	var calls int32
	withFEMATestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	})

	c := NewFEMAClient()
	_, err := c.Count(context.Background(), 28, 41.6, -87.9, 42.0, -87.5)
	if err == nil {
		t.Fatal("expected an error for a 400 response")
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 call (no retry on a non-retryable 4xx), got %d", calls)
	}
}

func TestQueryLayer_SimplePagination(t *testing.T) {
	// total=25, pageSize=10 -> pages of 10, 10, 5.
	var calls int32
	withFEMATestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("returnCountOnly") == "true" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"count":25}`))
			return
		}
		atomic.AddInt32(&calls, 1)
		n, _ := strconv.Atoi(r.URL.Query().Get("resultRecordCount"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("resultOffset"))
		remaining := 25 - offset
		if n > remaining {
			n = remaining
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(featureCollectionJSON(n)))
	})

	c := NewFEMAClient()
	features, err := c.QueryLayer(context.Background(), 28, 41.6, -87.9, 42.0, -87.5, "FLD_ZONE", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(features) != 25 {
		t.Fatalf("expected 25 features total, got %d", len(features))
	}
	if calls != 3 {
		t.Fatalf("expected exactly 3 page requests (10+10+5), got %d", calls)
	}
}

func TestQueryLayer_ShrinksOnRetryableFailureThenSucceeds(t *testing.T) {
	withNoFEMABackoffWait(t)

	// total=20. First window (offset 0, size 10) fails until shrunk to <= 5.
	withFEMATestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("returnCountOnly") == "true" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"count":20}`))
			return
		}
		n, _ := strconv.Atoi(r.URL.Query().Get("resultRecordCount"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("resultOffset"))
		if offset == 0 && n > 5 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("too large"))
			return
		}
		remaining := 20 - offset
		if n > remaining {
			n = remaining
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(featureCollectionJSON(n)))
	})

	c := NewFEMAClient()
	features, err := c.QueryLayer(context.Background(), 28, 41.6, -87.9, 42.0, -87.5, "FLD_ZONE", 10)
	if err != nil {
		t.Fatalf("expected success after shrinking, got error: %v", err)
	}
	if len(features) != 20 {
		t.Fatalf("expected 20 features total, got %d", len(features))
	}
}

func TestQueryLayer_FailsClearlyBelowMinPageSize(t *testing.T) {
	withNoFEMABackoffWait(t)

	withFEMATestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("returnCountOnly") == "true" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"count":10}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("always too large"))
	})

	c := NewFEMAClient()
	_, err := c.QueryLayer(context.Background(), 28, 41.6, -87.9, 42.0, -87.5, "FLD_ZONE", 10)
	if err == nil {
		t.Fatal("expected an error when every window size fails, even at the minimum")
	}
}

func TestQueryLayer_BacksOffOnRateLimitWithoutShrinking(t *testing.T) {
	withNoFEMABackoffWait(t)

	// The first two page requests are rate limited; the third succeeds. The
	// requested page size must stay the same across all three (429 must not
	// trigger window shrinking, only backoff-and-retry).
	var pageCalls int32
	var sawSizes []string
	withFEMATestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("returnCountOnly") == "true" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"count":10}`))
			return
		}
		sawSizes = append(sawSizes, r.URL.Query().Get("resultRecordCount"))
		n := atomic.AddInt32(&pageCalls, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("rate limited"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(featureCollectionJSON(10)))
	})

	c := NewFEMAClient()
	features, err := c.QueryLayer(context.Background(), 28, 41.6, -87.9, 42.0, -87.5, "FLD_ZONE", 50)
	if err != nil {
		t.Fatalf("expected success after backing off on 429s, got error: %v", err)
	}
	if len(features) != 10 {
		t.Fatalf("expected 10 features, got %d", len(features))
	}
	if pageCalls != 3 {
		t.Fatalf("expected exactly 3 page requests (2 rate limited + 1 success), got %d", pageCalls)
	}
	for _, s := range sawSizes {
		if s != "50" {
			t.Fatalf("expected resultRecordCount to stay 50 across 429 retries (no shrinking), saw %v", sawSizes)
		}
	}
}

func TestQueryLayer_GivesUpAfterMaxRateLimitRetries(t *testing.T) {
	withNoFEMABackoffWait(t)

	withFEMATestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("returnCountOnly") == "true" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"count":10}`))
			return
		}
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limited"))
	})

	c := NewFEMAClient()
	_, err := c.QueryLayer(context.Background(), 28, 41.6, -87.9, 42.0, -87.5, "FLD_ZONE", 50)
	if err == nil {
		t.Fatal("expected an error when every retry attempt is rate limited")
	}
}

func TestQueryLayer_DoesNotShrinkOnNonRetryableError(t *testing.T) {
	var calls int32
	withFEMATestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("returnCountOnly") == "true" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"count":10}`))
			return
		}
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	})

	c := NewFEMAClient()
	_, err := c.QueryLayer(context.Background(), 28, 41.6, -87.9, 42.0, -87.5, "FLD_ZONE", 10)
	if err == nil {
		t.Fatal("expected an error for a 400 response")
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 call (no shrink-retry on a non-retryable 4xx), got %d", calls)
	}
}

func TestQueryLayer_OrdersByObjectIDForStablePagination(t *testing.T) {
	var gotOrderBy []string
	withFEMATestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("returnCountOnly") == "true" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"count":15}`))
			return
		}
		gotOrderBy = append(gotOrderBy, r.URL.Query().Get("orderByFields"))
		n, _ := strconv.Atoi(r.URL.Query().Get("resultRecordCount"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("resultOffset"))
		remaining := 15 - offset
		if n > remaining {
			n = remaining
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(featureCollectionJSON(n)))
	})

	c := NewFEMAClient()
	_, err := c.QueryLayer(context.Background(), 28, 41.6, -87.9, 42.0, -87.5, "FLD_ZONE", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotOrderBy) == 0 {
		t.Fatal("expected at least one page request")
	}
	for _, ob := range gotOrderBy {
		if ob != "OBJECTID" {
			t.Fatalf("expected every page request to set orderByFields=OBJECTID for stable pagination, got %q", ob)
		}
	}
}

func TestFEMAClient_BuildsCorrectBoundingBoxParameter(t *testing.T) {
	var gotGeometry string
	withFEMATestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("returnCountOnly") == "true" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"count":1}`))
			return
		}
		gotGeometry = r.URL.Query().Get("geometry")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(featureCollectionJSON(1)))
	})

	c := NewFEMAClient()
	_, err := c.QueryLayer(context.Background(), 28, 41.6445, -87.9401, 42.0230, -87.5241, "FLD_ZONE", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// geometry must be "west,south,east,north" (Esri envelope order), not
	// the south/west/north/east order the function's own parameters use.
	want := url.QueryEscape(fmt.Sprintf("%f,%f,%f,%f", -87.9401, 41.6445, -87.5241, 42.0230))
	got := url.QueryEscape(gotGeometry)
	if got != want {
		t.Errorf("geometry param = %q, want %q (west,south,east,north order)", gotGeometry, want)
	}
}
