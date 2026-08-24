package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
)

type mockGeocoder struct {
	geocodeFunc func(ctx context.Context, query string) ([]clients.GeocodeCandidate, error)
}

func (m *mockGeocoder) Geocode(ctx context.Context, query string) ([]clients.GeocodeCandidate, error) {
	return m.geocodeFunc(ctx, query)
}

func TestGetHealth(t *testing.T) {
	srv := api.NewServer(nil)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res api.HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Status != "ok" {
		t.Fatalf("expected status ok, got %q", res.Status)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	srv := api.NewServer(nil)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestGeocodeAddress_Success(t *testing.T) {
	mockGeo := &mockGeocoder{
		geocodeFunc: func(_ context.Context, query string) ([]clients.GeocodeCandidate, error) {
			if query != "233 S Wacker Dr" {
				t.Errorf("expected query '233 S Wacker Dr', got %q", query)
			}
			return []clients.GeocodeCandidate{
				{
					FormattedAddress: "233 S Wacker Dr, Chicago, IL 60606",
					Lat:              41.878876,
					Lon:              -87.635915,
					Rank:             0,
				},
			}, nil
		},
	}

	logger := slog.New(slog.DiscardHandler)
	srv := api.NewServer(logger, api.WithGeocoder(mockGeo))
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/geocode?q=233+S+Wacker+Dr", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res api.GeocodeResponse
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode geocode response: %v", err)
	}

	if len(res.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(res.Candidates))
	}
	candidate := res.Candidates[0]
	if candidate.FormattedAddress != "233 S Wacker Dr, Chicago, IL 60606" || candidate.Coordinates.Lat != 41.878876 || candidate.Coordinates.Lon != -87.635915 || candidate.Rank != 0 {
		t.Errorf("unexpected candidate result: %+v", candidate)
	}
}

func TestGeocodeAddress_EmptyQuery(t *testing.T) {
	srv := api.NewServer(nil)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/geocode?q=%20%20", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for empty query, got %d", w.Code)
	}

	var errResp api.Error
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %q", errResp.Code)
	}
}

func TestGeocodeAddress_MissingGeocoder(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	srv := api.NewServer(logger)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/geocode?q=Chicago", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", w.Code)
	}

	var errResp api.Error
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "SERVICE_UNAVAILABLE" {
		t.Errorf("expected error code SERVICE_UNAVAILABLE, got %q", errResp.Code)
	}
}

func TestGeocodeAddress_GeocodeError(t *testing.T) {
	mockGeo := &mockGeocoder{
		geocodeFunc: func(_ context.Context, _ string) ([]clients.GeocodeCandidate, error) {
			return nil, errors.New("upstream timeout")
		},
	}

	logger := slog.New(slog.DiscardHandler)
	srv := api.NewServer(logger, api.WithGeocoder(mockGeo))
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/geocode?q=Chicago", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", w.Code)
	}

	var errResp api.Error
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "GEOCODING_ERROR" {
		t.Errorf("expected error code GEOCODING_ERROR, got %q", errResp.Code)
	}
}

type mockBoundaryChecker struct {
	within bool
	err    error
}

func (m *mockBoundaryChecker) IsWithinCityBoundary(_ context.Context, _, _ float64) (bool, error) {
	return m.within, m.err
}

type mockScoreEngine struct {
	compositeScore float32
	factors        map[string]api.ScoreFactorResult
	err            error
	capturedParams api.ScoreEngineParams
}

func (m *mockScoreEngine) Score(_ context.Context, params api.ScoreEngineParams) (float32, map[string]api.ScoreFactorResult, error) {
	m.capturedParams = params
	if m.err != nil {
		return 0, nil, m.err
	}
	return m.compositeScore, m.factors, nil
}

func fullFactorSet(score float32) map[string]api.ScoreFactorResult {
	s := score
	factors := map[string]api.ScoreFactorResult{}
	for _, name := range []string{"transit", "bike", "grocery", "thirdPlace", "green", "noise", "safety", "flood", "solar"} {
		factors[name] = api.ScoreFactorResult{Score: &s}
	}
	return factors
}

func newScoredServer(engine *mockScoreEngine, boundary *mockBoundaryChecker, geo *mockGeocoder) *api.Server {
	logger := slog.New(slog.DiscardHandler)
	var opts []api.ServerOption
	opts = append(opts, api.WithScoreEngine(engine), api.WithBoundaryChecker(boundary))
	if geo != nil {
		opts = append(opts, api.WithGeocoder(geo))
	}
	return api.NewServer(logger, opts...)
}

func TestScoreAddress_SuccessWithPreResolvedOrigin(t *testing.T) {
	engine := &mockScoreEngine{compositeScore: 82.5, factors: fullFactorSet(80.0)}
	boundary := &mockBoundaryChecker{within: true}
	srv := newScoredServer(engine, boundary, nil)
	router := api.NewRouter(srv)

	body := `{"origin": {"lat": 41.8781, "lon": -87.6298}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var res api.ScoreResponse
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.CompositeScore != 82.5 {
		t.Errorf("expected compositeScore 82.5, got %v", res.CompositeScore)
	}
	if res.Factors.Transit.Score == nil || *res.Factors.Transit.Score != 80.0 {
		t.Errorf("expected transit score 80.0, got %v", res.Factors.Transit.Score)
	}
	if res.Factors.Commute != nil {
		t.Errorf("expected commute absent with no destination, got %+v", res.Factors.Commute)
	}
	if res.BenchmarkCommutes == nil || len(res.BenchmarkCommutes) != 0 {
		t.Errorf("expected an empty (not nil) benchmarkCommutes array, got %v", res.BenchmarkCommutes)
	}

	if engine.capturedParams.Origin.Lat != 41.8781 || engine.capturedParams.Origin.Lon != -87.6298 {
		t.Errorf("unexpected origin passed to engine: %+v", engine.capturedParams.Origin)
	}
}

func TestScoreAddress_SuccessWithDestinationIncludesCommute(t *testing.T) {
	factors := fullFactorSet(80.0)
	factors["commute"] = api.ScoreFactorResult{Degraded: true}
	engine := &mockScoreEngine{compositeScore: 82.5, factors: factors}
	boundary := &mockBoundaryChecker{within: true}
	srv := newScoredServer(engine, boundary, nil)
	router := api.NewRouter(srv)

	body := `{"origin": {"lat": 41.8781, "lon": -87.6298}, "destinationCoordinates": {"lat": 41.8858, "lon": -87.6353}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var res api.ScoreResponse
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.Factors.Commute == nil {
		t.Fatal("expected commute present when a destination is supplied")
	}
	if !res.Factors.Commute.Degraded {
		t.Errorf("expected commute degraded (no live OTP2 client yet), got not degraded")
	}

	if engine.capturedParams.DestinationCoordinates == nil ||
		engine.capturedParams.DestinationCoordinates.Lat != 41.8858 ||
		engine.capturedParams.DestinationCoordinates.Lon != -87.6353 {
		t.Errorf("unexpected destination passed to engine: %+v", engine.capturedParams.DestinationCoordinates)
	}
}

func TestScoreAddress_GeocodesAddressPickingBestRankedCandidate(t *testing.T) {
	geo := &mockGeocoder{
		geocodeFunc: func(_ context.Context, query string) ([]clients.GeocodeCandidate, error) {
			if query != "233 S Wacker Dr, Chicago, IL" {
				t.Errorf("unexpected geocode query: %q", query)
			}
			return []clients.GeocodeCandidate{
				{Lat: 41.0, Lon: -87.0, Rank: 2},
				{Lat: 41.8781, Lon: -87.6298, Rank: 0},
				{Lat: 41.5, Lon: -87.5, Rank: 1},
			}, nil
		},
	}
	engine := &mockScoreEngine{compositeScore: 50.0, factors: fullFactorSet(50.0)}
	boundary := &mockBoundaryChecker{within: true}
	srv := newScoredServer(engine, boundary, geo)
	router := api.NewRouter(srv)

	body := `{"address": "233 S Wacker Dr, Chicago, IL"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if engine.capturedParams.Origin.Lat != 41.8781 || engine.capturedParams.Origin.Lon != -87.6298 {
		t.Errorf("expected the rank-0 candidate to be selected, got %+v", engine.capturedParams.Origin)
	}
}

func TestScoreAddress_NeitherAddressNorOrigin(t *testing.T) {
	engine := &mockScoreEngine{}
	boundary := &mockBoundaryChecker{within: true}
	srv := newScoredServer(engine, boundary, nil)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
	var errResp api.Error
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %q", errResp.Code)
	}
}

func TestScoreAddress_BothAddressAndOrigin(t *testing.T) {
	engine := &mockScoreEngine{}
	boundary := &mockBoundaryChecker{within: true}
	srv := newScoredServer(engine, boundary, nil)
	router := api.NewRouter(srv)

	body := `{"address": "233 S Wacker Dr", "origin": {"lat": 41.8781, "lon": -87.6298}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestScoreAddress_BothDestinationAndDestinationCoordinates(t *testing.T) {
	engine := &mockScoreEngine{}
	boundary := &mockBoundaryChecker{within: true}
	srv := newScoredServer(engine, boundary, nil)
	router := api.NewRouter(srv)

	body := `{"origin": {"lat": 41.8781, "lon": -87.6298}, "destination": "1 N State St", "destinationCoordinates": {"lat": 41.8858, "lon": -87.6353}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestScoreAddress_InvalidJSON(t *testing.T) {
	engine := &mockScoreEngine{}
	boundary := &mockBoundaryChecker{within: true}
	srv := newScoredServer(engine, boundary, nil)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(`{not json`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestScoreAddress_EngineNotConfigured(t *testing.T) {
	srv := api.NewServer(nil)
	router := api.NewRouter(srv)

	body := `{"origin": {"lat": 41.8781, "lon": -87.6298}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", w.Code)
	}
	var errResp api.Error
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "SERVICE_UNAVAILABLE" {
		t.Errorf("expected SERVICE_UNAVAILABLE, got %q", errResp.Code)
	}
}

func TestScoreAddress_BoundaryCheckerNotConfigured(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	engine := &mockScoreEngine{}
	srv := api.NewServer(logger, api.WithScoreEngine(engine))
	router := api.NewRouter(srv)

	body := `{"origin": {"lat": 41.8781, "lon": -87.6298}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", w.Code)
	}
}

func TestScoreAddress_AddressGeocoderNotConfigured(t *testing.T) {
	engine := &mockScoreEngine{}
	boundary := &mockBoundaryChecker{within: true}
	srv := newScoredServer(engine, boundary, nil)
	router := api.NewRouter(srv)

	body := `{"address": "233 S Wacker Dr"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", w.Code)
	}
}

func TestScoreAddress_AddressNotFound(t *testing.T) {
	geo := &mockGeocoder{
		geocodeFunc: func(context.Context, string) ([]clients.GeocodeCandidate, error) { return nil, nil },
	}
	engine := &mockScoreEngine{}
	boundary := &mockBoundaryChecker{within: true}
	srv := newScoredServer(engine, boundary, geo)
	router := api.NewRouter(srv)

	body := `{"address": "some nonexistent place"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
	var errResp api.Error
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "ADDRESS_NOT_FOUND" {
		t.Errorf("expected ADDRESS_NOT_FOUND, got %q", errResp.Code)
	}
}

func TestScoreAddress_GeocodingError(t *testing.T) {
	geo := &mockGeocoder{
		geocodeFunc: func(context.Context, string) ([]clients.GeocodeCandidate, error) {
			return nil, errors.New("upstream timeout")
		},
	}
	engine := &mockScoreEngine{}
	boundary := &mockBoundaryChecker{within: true}
	srv := newScoredServer(engine, boundary, geo)
	router := api.NewRouter(srv)

	body := `{"address": "233 S Wacker Dr"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", w.Code)
	}
	var errResp api.Error
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "GEOCODING_ERROR" {
		t.Errorf("expected GEOCODING_ERROR, got %q", errResp.Code)
	}
}

func TestScoreAddress_OutOfCoverage(t *testing.T) {
	engine := &mockScoreEngine{}
	boundary := &mockBoundaryChecker{within: false}
	srv := newScoredServer(engine, boundary, nil)
	router := api.NewRouter(srv)

	body := `{"origin": {"lat": 42.5, "lon": -88.5}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
	var errResp api.Error
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "OUT_OF_COVERAGE" {
		t.Errorf("expected OUT_OF_COVERAGE, got %q", errResp.Code)
	}
}

func TestScoreAddress_BoundaryCheckError(t *testing.T) {
	engine := &mockScoreEngine{}
	boundary := &mockBoundaryChecker{err: errors.New("db connection lost")}
	srv := newScoredServer(engine, boundary, nil)
	router := api.NewRouter(srv)

	body := `{"origin": {"lat": 41.8781, "lon": -87.6298}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", w.Code)
	}
}

func TestScoreAddress_EngineError(t *testing.T) {
	engine := &mockScoreEngine{err: errors.New("all active factor weights sum to zero")}
	boundary := &mockBoundaryChecker{within: true}
	srv := newScoredServer(engine, boundary, nil)
	router := api.NewRouter(srv)

	body := `{"origin": {"lat": 41.8781, "lon": -87.6298}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/score", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
	var errResp api.Error
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Code != "SCORING_ERROR" {
		t.Errorf("expected SCORING_ERROR, got %q", errResp.Code)
	}
}

func TestGetIsochrones_NotImplemented(t *testing.T) {
	srv := api.NewServer(nil)
	router := api.NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/isochrones?lat=41.8781&lon=-87.6298", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected status 501, got %d", w.Code)
	}
}
