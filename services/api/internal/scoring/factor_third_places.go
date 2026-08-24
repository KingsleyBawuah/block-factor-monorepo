package scoring

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/clients"
	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/spatial"
)

// Third place scoring thresholds, radii, and cache/budget constants (PRD §5.4).
const (
	// ThirdPlaceQueryRadiusMeters is 0.5 miles (804.672 m) -- the overall
	// POI search window.
	ThirdPlaceQueryRadiusMeters = 804.672
	// ThirdPlaceCloseThresholdMeters is 0.4 miles (643.738 m) -- the tighter
	// radius required for the 100 pt benchmark.
	ThirdPlaceCloseThresholdMeters = 643.738

	ThirdPlaceScorePerfect = 100.0
	ThirdPlaceScoreGood    = 75.0
	ThirdPlaceScoreMinimal = 40.0

	// ThirdPlaceMinIndependentCafesForPerfect/Good are the cafe-count
	// thresholds for the 100 pt and 75 pt tiers respectively.
	ThirdPlaceMinIndependentCafesForPerfect = 3
	ThirdPlaceMinIndependentCafesForGood    = 2

	// ThirdPlaceCacheTTL is 30 days -- Google's Places (New) ToS maximum
	// cache lifetime for this field set.
	ThirdPlaceCacheTTL = 30 * 24 * time.Hour

	// thirdPlaceGridCellMeters is the 400m grid cell size cache reads/writes
	// are bucketed by (PRIVACY_AND_DATA_HANDLING.md §3: keyed by grid cell,
	// never raw address), so nearby addresses share a single cached (and
	// paid) API fetch.
	thirdPlaceGridCellMeters = 400.0

	// thirdPlaceFillRadiusMeters is the radius a cache-fill actually
	// searches, centered on the grid cell's center rather than the specific
	// requesting address (see thirdPlaceGridCellCenter): large enough that
	// any address anywhere in the 400m cell -- even at its corner -- has
	// its full ThirdPlaceQueryRadiusMeters query window covered by this one
	// shared fetch. That's the cell's half-diagonal (400 / sqrt(2)) plus
	// the query radius itself. Without this, two addresses sharing a cell
	// but sitting near opposite edges of it could see different real POIs,
	// yet the second one silently reuses the first one's narrower fetch.
	thirdPlaceFillRadiusMeters = thirdPlaceGridCellMeters/math.Sqrt2 + ThirdPlaceQueryRadiusMeters
)

// thirdPlaceIncludedTypes are the Places (New) Table A type values searched
// for S_third_place: cafes (independence-classified below) and fitness
// venues, fetched in a single combined request to conserve the daily call
// budget. "coffee_shop" is a distinct Table A type from "cafe" -- both are
// searched and treated identically by isCafeType.
var thirdPlaceIncludedTypes = []string{"cafe", "coffee_shop", "gym", "fitness_center", "yoga_studio"}

// thirdPlaceChainBrands is a maintained exclusion list of chain coffee
// brands used to approximate "independent" (PRD §5.4: Google Places data
// carries no such flag). Matched as a substring against the place's display
// name, normalized by isChainCafeBrand (lowercased, punctuation stripped),
// at cache-fill time. Entries are bare brand tokens rather than full
// storefront names, since Google Places display names vary by location
// ("Peet's Coffee" vs. plain "Peets"). Not exhaustive -- extend as new
// chains are observed slipping through in real Chicago data.
var thirdPlaceChainBrands = []string{
	"starbucks",
	"dunkin",
	"peet",
	"caribou",
	"tim hortons",
	"philz",
	"dutch bros",
	"coffee bean & tea leaf",
	"biggby",
	"pret a manger",
	"panera",
	"capital one cafe",
	"einstein bros",
	"au bon pain",
}

// ThirdPlacePoi is a classified, cacheable third-place POI. Independence is
// resolved once at cache-fill time (not on every read) per the PRD's own
// wording of the heuristic.
type ThirdPlacePoi struct {
	Name              string
	PrimaryType       string // "cafe", "coffee_shop", "gym", "fitness_center", or "yoga_studio"
	IsIndependentCafe bool
	Lat               float64
	Lon               float64
}

// PlacesQuerier is the subset of clients.PlacesClient the third-place
// calculator depends on, kept as an interface so tests can fake it without
// an HTTP server.
type PlacesQuerier interface {
	SearchNearby(ctx context.Context, lat, lon, radiusMeters float64, includedTypes []string) ([]clients.Place, error)
}

// ThirdPlaceCache is the subset of *cache.Cache the third-place calculator
// depends on.
type ThirdPlaceCache interface {
	Get(ctx context.Context, key string, dest any) (ok bool, err error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
}

// PlacesBudget enforces a daily cap on Google Places API calls so a bug or
// traffic spike can't run up unbounded third-party spend -- tripping it
// degrades S_third_place to null for the remainder of the day rather than
// erroring the whole score request.
type PlacesBudget interface {
	// IncrementAndCheck atomically records one more call attempt for today
	// and reports whether that attempt is still within dailyCapCalls.
	IncrementAndCheck(ctx context.Context, dailyCapCalls int) (withinBudget bool, err error)
}

// DBPlacesBudget persists the daily call count in Postgres (places_api_budget)
// rather than in-process memory: an in-memory counter would silently reset
// on every restart/redeploy, reopening the day's spend cap exactly when a
// restart-triggered retry storm makes that most dangerous. A Postgres row
// survives restarts and is shared correctly across multiple server instances.
type DBPlacesBudget struct {
	db spatial.QueryRowProvider
}

// NewDBPlacesBudget creates a new DBPlacesBudget instance.
func NewDBPlacesBudget(db spatial.QueryRowProvider) *DBPlacesBudget {
	return &DBPlacesBudget{db: db}
}

// IncrementAndCheck atomically increments today's call_count in
// places_api_budget (creating today's row on first use), then compares the
// post-increment count against dailyCapCalls. The budget day is evaluated
// in America/Chicago (this app's only service area) rather than the
// database server's own timezone (UTC in this project's docker-compose
// setup), so the cap rolls over at Chicago midnight -- not, confusingly,
// at 6-7pm local time.
func (b *DBPlacesBudget) IncrementAndCheck(ctx context.Context, dailyCapCalls int) (bool, error) {
	if b.db == nil {
		return false, errors.New("scoring: nil database connection")
	}

	query := `
		INSERT INTO places_api_budget (budget_date, call_count)
		VALUES ((now() AT TIME ZONE 'America/Chicago')::date, 1)
		ON CONFLICT (budget_date) DO UPDATE SET call_count = places_api_budget.call_count + 1
		RETURNING call_count;
	`

	var callCount int
	if err := b.db.QueryRow(ctx, query).Scan(&callCount); err != nil {
		return false, fmt.Errorf("scoring: increment places api budget: %w", err)
	}

	return callCount <= dailyCapCalls, nil
}

// thirdPlaceFillResult is the singleflight payload for a cache-fill
// attempt -- degraded is distinct from "pois successfully fetched, zero
// results", which is a real (non-degraded) score of 0.
type thirdPlaceFillResult struct {
	pois     []ThirdPlacePoi
	degraded bool
}

// ThirdPlaceCalculator calculates the S_third_place livability factor.
type ThirdPlaceCalculator struct {
	places        PlacesQuerier
	cache         ThirdPlaceCache
	budget        PlacesBudget
	dailyCapCalls int
	sf            singleflight.Group
}

// NewThirdPlaceCalculator creates a new ThirdPlaceCalculator. dailyCapCalls
// is the self-imposed daily Google Places call ceiling (GOOGLE_PLACES_DAILY_BUDGET_CALLS).
func NewThirdPlaceCalculator(places PlacesQuerier, cache ThirdPlaceCache, budget PlacesBudget, dailyCapCalls int) *ThirdPlaceCalculator {
	return &ThirdPlaceCalculator{
		places:        places,
		cache:         cache,
		budget:        budget,
		dailyCapCalls: dailyCapCalls,
	}
}

// Name returns FactorThirdPlace.
func (c *ThirdPlaceCalculator) Name() FactorName {
	return FactorThirdPlace
}

// Calculate computes S_third_place for the given origin coordinate.
func (c *ThirdPlaceCalculator) Calculate(ctx context.Context, params ScoreParams) (FactorResult, error) {
	if c.places == nil || c.cache == nil || c.budget == nil {
		return FactorResult{Name: FactorThirdPlace, Degraded: true}, errors.New("scoring: third place calculator not configured")
	}

	lat, lon := params.Origin.Lat, params.Origin.Lon
	pois, degraded, err := c.resolvePois(ctx, thirdPlaceGridKey(lat, lon), lat, lon)
	if err != nil {
		return FactorResult{Name: FactorThirdPlace, Degraded: true}, err
	}
	if degraded {
		return FactorResult{Name: FactorThirdPlace, Degraded: true}, nil
	}

	score, minDistance, contributing := CalculateThirdPlaceScore(pois, lat, lon)
	return FactorResult{
		Name:             FactorThirdPlace,
		Score:            &score,
		DistanceMeters:   minDistance,
		ContributingPois: contributing,
		Degraded:         false,
	}, nil
}

// resolvePois returns the third-place POIs for a grid cell, filling the
// cache on a miss. Concurrent misses for the same cell are deduplicated via
// singleflight so only one goroutine per process actually calls Places or
// spends budget for a given cell at a time.
func (c *ThirdPlaceCalculator) resolvePois(ctx context.Context, gridKey string, lat, lon float64) (pois []ThirdPlacePoi, degraded bool, err error) {
	var cached []ThirdPlacePoi
	ok, err := c.cache.Get(ctx, gridKey, &cached)
	if err != nil {
		return nil, true, err
	}
	if ok {
		return cached, false, nil
	}

	v, err, _ := c.sf.Do(gridKey, func() (any, error) {
		// Re-check: another goroutine may have filled this cell between our
		// miss above and acquiring the singleflight slot.
		var recheck []ThirdPlacePoi
		if ok, err := c.cache.Get(ctx, gridKey, &recheck); err != nil {
			return thirdPlaceFillResult{}, err
		} else if ok {
			return thirdPlaceFillResult{pois: recheck}, nil
		}

		withinBudget, err := c.budget.IncrementAndCheck(ctx, c.dailyCapCalls)
		if err != nil {
			return thirdPlaceFillResult{}, err
		}
		if !withinBudget {
			return thirdPlaceFillResult{degraded: true}, nil
		}

		// Search around the cell's own center, not the requesting address --
		// see thirdPlaceFillRadiusMeters for why.
		fillLat, fillLon := thirdPlaceGridCellCenter(lat, lon)
		fetched, err := c.places.SearchNearby(ctx, fillLat, fillLon, thirdPlaceFillRadiusMeters, thirdPlaceIncludedTypes)
		if err != nil {
			return thirdPlaceFillResult{}, err
		}

		filled := classifyThirdPlacePois(fetched)
		if err := c.cache.Set(ctx, gridKey, filled, ThirdPlaceCacheTTL); err != nil {
			return thirdPlaceFillResult{}, err
		}
		return thirdPlaceFillResult{pois: filled}, nil
	})
	if err != nil {
		return nil, true, err
	}

	result, _ := v.(thirdPlaceFillResult)
	return result.pois, result.degraded, nil
}

// isCafeType reports whether primaryType is either of the two Table A
// types PRD §5.4's cafe benchmark covers -- Google Places (New) treats
// "coffee_shop" as distinct from "cafe", but the PRD makes no such
// distinction.
func isCafeType(primaryType string) bool {
	return primaryType == "cafe" || primaryType == "coffee_shop"
}

// isFitnessType reports whether primaryType is one of the Table A types
// PRD §5.4's "fitness center/studio" benchmark covers.
func isFitnessType(primaryType string) bool {
	return primaryType == "gym" || primaryType == "fitness_center" || primaryType == "yoga_studio"
}

// classifyThirdPlacePois normalizes raw Places results and resolves cafe
// independence once, at cache-fill time (PRD §5.4).
func classifyThirdPlacePois(places []clients.Place) []ThirdPlacePoi {
	pois := make([]ThirdPlacePoi, 0, len(places))
	for _, p := range places {
		pois = append(pois, ThirdPlacePoi{
			Name:              p.Name,
			PrimaryType:       p.PrimaryType,
			IsIndependentCafe: isCafeType(p.PrimaryType) && !isChainCafeBrand(p.Name),
			Lat:               p.Lat,
			Lon:               p.Lon,
		})
	}
	return pois
}

// thirdPlaceNamePunctuation is stripped from a display name before chain
// matching, so e.g. "Peet's Coffee" and "Peets Coffee" both normalize to
// "peets coffee" and match the same bare "peet" brand token.
var thirdPlaceNamePunctuation = strings.NewReplacer("'", "", "’", "", ".", "", ",", "")

func isChainCafeBrand(name string) bool {
	normalized := thirdPlaceNamePunctuation.Replace(strings.ToLower(name))
	for _, brand := range thirdPlaceChainBrands {
		if strings.Contains(normalized, brand) {
			return true
		}
	}
	return false
}

// thirdPlaceGridCellIndices returns the integer 400m grid cell coordinates
// containing (lat, lon), plus the local meters-per-degree-longitude
// conversion at that latitude (longitude degrees shrink toward the poles),
// shared by thirdPlaceGridKey and thirdPlaceGridCellCenter so the two never
// drift out of sync with each other.
func thirdPlaceGridCellIndices(lat, lon float64) (latCell, lonCell float64, metersPerDegreeLon float64) {
	const metersPerDegreeLat = 111_320.0
	metersPerDegreeLon = metersPerDegreeLat * math.Cos(lat*math.Pi/180)

	latCell = math.Floor(lat * metersPerDegreeLat / thirdPlaceGridCellMeters)
	lonCell = math.Floor(lon * metersPerDegreeLon / thirdPlaceGridCellMeters)
	return latCell, lonCell, metersPerDegreeLon
}

// thirdPlaceGridKey buckets a coordinate into the 400m grid cell used to key
// the Google Places cache, so nearby addresses within the same cell share a
// single cached (and paid) API fetch. Uses a flat equirectangular
// approximation rather than PostGIS -- accurate enough for city-scale cache
// bucketing, and keeps this pure Go with no DB round trip.
func thirdPlaceGridKey(lat, lon float64) string {
	latCell, lonCell, _ := thirdPlaceGridCellIndices(lat, lon)
	return fmt.Sprintf("places:third_place:%d_%d", int64(latCell), int64(lonCell))
}

// thirdPlaceGridCellCenter returns the coordinate at the exact center of
// the 400m grid cell containing (lat, lon). A cache-fill searches around
// this center point -- combined with thirdPlaceFillRadiusMeters -- rather
// than the specific requesting address, so every address sharing a cell
// sees the same, fully-covering cached result regardless of where in the
// cell it happens to fall.
func thirdPlaceGridCellCenter(lat, lon float64) (centerLat, centerLon float64) {
	const metersPerDegreeLat = 111_320.0
	latCell, lonCell, metersPerDegreeLon := thirdPlaceGridCellIndices(lat, lon)

	centerLat = (latCell + 0.5) * thirdPlaceGridCellMeters / metersPerDegreeLat
	centerLon = (lonCell + 0.5) * thirdPlaceGridCellMeters / metersPerDegreeLon
	return centerLat, centerLon
}

// haversineDistanceMeters computes the great-circle distance between two
// coordinates. Third-place POIs are fetched live from Google and cached per
// grid cell rather than stored in PostGIS, so unlike the DB-backed factors
// (which get distance for free from ST_Distance(geography)), this needs a
// pure-Go equivalent computed at read time against the specific request
// origin.
func haversineDistanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusMeters = 6_371_000.0
	toRad := func(deg float64) float64 { return deg * math.Pi / 180 }

	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusMeters * c
}

type thirdPlaceCandidate struct {
	poi            ThirdPlacePoi
	distanceMeters float64
}

func closestCandidate(candidates []thirdPlaceCandidate) *thirdPlaceCandidate {
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].distanceMeters < candidates[j].distanceMeters })
	return &candidates[0]
}

// CalculateThirdPlaceScore computes the third-place livability factor based
// on PRD §5.4:
//   - >= 3 independent cafes AND >= 1 fitness venue within <= 0.4 mi: 100 pts.
//   - >= 2 independent cafes OR >= 1 fitness venue within <= 0.5 mi (if the
//     100 pt benchmark isn't met): 75 pts.
//   - At least 1 independent cafe or fitness venue within <= 0.5 mi: 40 pts.
//   - Otherwise: 0 pts.
func CalculateThirdPlaceScore(pois []ThirdPlacePoi, originLat, originLon float64) (score float32, minDistance *float32, contributing []api.ContributingPoi) {
	var independentCafesClose, independentCafesWide []thirdPlaceCandidate
	var fitnessClose, fitnessWide []thirdPlaceCandidate

	for _, p := range pois {
		d := haversineDistanceMeters(originLat, originLon, p.Lat, p.Lon)
		if d > ThirdPlaceQueryRadiusMeters {
			continue
		}
		candidate := thirdPlaceCandidate{poi: p, distanceMeters: d}

		switch {
		case isCafeType(p.PrimaryType) && p.IsIndependentCafe:
			independentCafesWide = append(independentCafesWide, candidate)
			if d <= ThirdPlaceCloseThresholdMeters {
				independentCafesClose = append(independentCafesClose, candidate)
			}
		case isFitnessType(p.PrimaryType):
			fitnessWide = append(fitnessWide, candidate)
			if d <= ThirdPlaceCloseThresholdMeters {
				fitnessClose = append(fitnessClose, candidate)
			}
		}
	}

	switch {
	case len(independentCafesClose) >= ThirdPlaceMinIndependentCafesForPerfect && len(fitnessClose) >= 1:
		score = ThirdPlaceScorePerfect
	case len(independentCafesWide) >= ThirdPlaceMinIndependentCafesForGood || len(fitnessWide) >= 1:
		score = ThirdPlaceScoreGood
	case len(independentCafesWide) >= 1 || len(fitnessWide) >= 1:
		score = ThirdPlaceScoreMinimal
	default:
		score = 0
	}

	closestCafe := closestCandidate(independentCafesWide)
	closestFitness := closestCandidate(fitnessWide)

	if closestCafe != nil {
		d := float32(math.Round(closestCafe.distanceMeters*10) / 10)
		contributing = append(contributing, api.ContributingPoi{Name: closestCafe.poi.Name, DistanceMeters: d})
		if minDistance == nil || d < *minDistance {
			minDistance = &d
		}
	}
	if closestFitness != nil {
		d := float32(math.Round(closestFitness.distanceMeters*10) / 10)
		contributing = append(contributing, api.ContributingPoi{Name: closestFitness.poi.Name, DistanceMeters: d})
		if minDistance == nil || d < *minDistance {
			minDistance = &d
		}
	}

	return score, minDistance, contributing
}
