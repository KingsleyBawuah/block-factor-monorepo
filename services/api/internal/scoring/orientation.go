package scoring

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/api"
)

var (
	// houseNumberRegex extracts leading digits from an address string.
	houseNumberRegex = regexp.MustCompile(`(?i)^\s*(\d+)`)

	// numberedStreetRegex identifies numbered East-West streets (e.g. "17th", "18th", "111th").
	numberedStreetRegex = regexp.MustCompile(`(?i)^\d+(?:st|nd|rd|th)$`)

	// diagonalNWSERegex identifies northwest-to-southeast diagonal avenues.
	diagonalNWSERegex = regexp.MustCompile(`(?i)\b(milwaukee|elston|clybourn|lincoln|northwest\s*hwy|south\s*chicago|indianapolis)\b`)

	// diagonalSWNERegex identifies southwest-to-northeast diagonal avenues.
	diagonalSWNERegex = regexp.MustCompile(`(?i)\b(archer|blue\s*island|ogden|columbus)\b`)

	// eastWestNames lists major named East-West thoroughfares in Chicago.
	eastWestNames = map[string]bool{
		"howard": true, "touhy": true, "pratt": true, "devon": true, "peterson": true,
		"bryn mawr": true, "foster": true, "lawrence": true, "wilson": true, "sunnyside": true,
		"montrose": true, "irving park": true, "grace": true, "waveland": true, "addison": true,
		"roscoe": true, "belmont": true, "school": true, "wellington": true, "diversey": true,
		"wrightwood": true, "fullerton": true, "belden": true, "webster": true, "armitage": true,
		"cortland": true, "north": true, "north ave": true, "division": true, "augusta": true,
		"chicago": true, "chicago ave": true, "superior": true, "huron": true, "erie": true,
		"ontario": true, "ohio": true, "grand": true, "illinois": true, "hubbard": true,
		"kinzie": true, "madison": true, "monroe": true, "adams": true, "jackson": true,
		"van buren": true, "congress": true, "ida b wells": true, "harrison": true, "polk": true,
		"taylor": true, "roosevelt": true, "cermak": true, "pershing": true, "garfield": true,
	}

	// streetSuffixWords are generic street-type words (both abbreviated and
	// full spellings, since real Nominatim results always spell these out --
	// "North Wells Street", never "N Wells St") that can immediately follow
	// a direction word without that direction word being part of the
	// street's own name.
	streetSuffixWords = map[string]bool{
		"st": true, "street": true,
		"ave": true, "avenue": true,
		"rd": true, "road": true,
		"blvd": true, "boulevard": true,
		"dr": true, "drive": true,
		"pkwy": true, "parkway": true,
		"pl": true, "place": true,
		"ln": true, "lane": true,
		"ct": true, "court": true,
		"way": true,
	}

	// northSouthNames lists major named North-South thoroughfares in Chicago.
	northSouthNames = map[string]bool{
		"state": true, "wabash": true, "michigan": true, "indiana": true, "prairie": true,
		"calumet": true, "clark": true, "dearborn": true, "lasalle": true, "wells": true,
		"franklin": true, "canal": true, "clinton": true, "jefferson": true, "desplaines": true,
		"halsted": true, "morgan": true, "racine": true, "loomis": true, "throop": true,
		"ashland": true, "paulina": true, "wood": true, "damen": true, "hoyne": true,
		"leavitt": true, "oakley": true, "western": true, "claremont": true, "bell": true,
		"campbell": true, "maplewood": true, "rockwell": true, "talman": true, "washtenaw": true,
		"fairfield": true, "california": true, "francisco": true, "sacramento": true, "albany": true,
		"troy": true, "kedzie": true, "sawyer": true, "spaulding": true, "homan": true,
		"st louis": true, "drake": true, "central park": true, "monticello": true, "lawndale": true,
		"millard": true, "ridgeway": true, "hamlin": true, "avers": true, "springfield": true,
		"harding": true, "pulaski": true, "karlov": true, "keeler": true, "tripp": true,
		"kildare": true, "kolin": true, "kostner": true, "kenneth": true, "kilbourn": true,
		"kolmar": true, "knox": true, "kilpatrick": true, "cicero": true, "keating": true,
		"lamon": true, "lavergne": true, "leclaire": true, "lawler": true, "central": true,
		"parkside": true, "major": true, "mango": true, "menard": true, "marmora": true,
		"mason": true, "mayfield": true, "austin": true, "meade": true, "moody": true,
		"melvina": true, "mobile": true, "merrimac": true, "natoma": true, "nashville": true,
		"natchez": true, "nagle": true, "narragansett": true, "oak park": true, "harlem": true,
	}
)

// StreetAxis represents the primary alignment of a street.
type StreetAxis int

// StreetAxis classifications for Chicago thoroughfares.
const (
	AxisEastWest StreetAxis = iota
	AxisNorthSouth
	AxisDiagonalNWSE
	AxisDiagonalSWNE
	AxisUnknown
)

// InferStreetFacingOrientation derives a building's primary street-facing window direction
// based on Chicago's 1909 Brennan address numbering system:
//
// 1. East-West streets:
//   - Odd house numbers are on the South side -> street-facing windows face North (N).
//   - Even house numbers are on the North side -> street-facing windows face South (S).
//
// 2. North-South streets:
//   - Odd house numbers are on the East side -> street-facing windows face West (W).
//   - Even house numbers are on the West side -> street-facing windows face East (E).
//
// 3. Diagonal NW-SE avenues (Milwaukee, Elston, Lincoln):
//   - Odd numbers face Northeast (NE); Even numbers face Southwest (SW).
//
// 4. Diagonal SW-NE avenues (Archer, Blue Island, Ogden):
//   - Odd numbers face Northwest (NW); Even numbers face Southeast (SE).
func InferStreetFacingOrientation(address string) api.UnitMetadataFacing {
	clean := strings.ToLower(strings.TrimSpace(address))
	if clean == "" {
		return api.S // Default fallback
	}
	clean = stripLeadingNonNumericSegments(clean)

	match := houseNumberRegex.FindStringSubmatch(clean)
	if len(match) < 2 {
		return api.S
	}

	houseNum, err := strconv.Atoi(match[1])
	if err != nil {
		return api.S
	}
	isOdd := houseNum%2 != 0

	// Bounded to the house-number segment plus the immediate next segment
	// (the actual street) before any classification runs. Real Nominatim
	// results carry the full admin hierarchy after the street (neighborhood,
	// city, township, county, ...), and Cook County's real "South Chicago
	// Township" (verified live in the Willis Tower geocode result) false-
	// matched diagonalNWSERegex's "south chicago" alternative (meant for
	// South Chicago Ave) when the unbounded remainder was searched.
	streetScope := scopeToStreetSegment(clean)

	axis := detectStreetAxis(streetScope)
	switch axis {
	case AxisEastWest:
		if isOdd {
			return api.N
		}
		return api.S
	case AxisNorthSouth:
		if isOdd {
			return api.W
		}
		return api.E
	case AxisDiagonalNWSE:
		if isOdd {
			return api.NE
		}
		return api.SW
	case AxisDiagonalSWNE:
		if isOdd {
			return api.NW
		}
		return api.SE
	default:
		// Default rule: check directional prefixes (e.g. "W 17th St" vs "N Halsted St")
		if isDirEastWestPrefix(streetScope) {
			if isOdd {
				return api.N
			}
			return api.S
		}
		if isOdd {
			return api.W
		}
		return api.E
	}
}

// stripLeadingNonNumericSegments drops any comma-separated segments before
// the first one that is purely a house number. Real Nominatim
// formattedAddress results prefix a landmark/POI name before the house
// number for well-known buildings -- e.g. "Willis Tower, 233, South Wacker
// Drive, Financial District, ..." for 233 S Wacker Dr (verified live
// against GET /api/v1/geocode) -- rather than starting with the number the
// way a plain, unnamed address does ("1400, North Wells Street, ...",
// also verified live). Without this, houseNumberRegex's `^\s*(\d+)` never
// matches a landmark-prefixed result, and every such address silently fell
// back to the default South/100 orientation. Returns the address
// unchanged if no purely-numeric segment is found, so the plain
// space-separated address strings this function's own tests already cover
// (e.g. "1801 W 18th St, Chicago, IL", which has no comma before the
// house number) fall through exactly as before.
func stripLeadingNonNumericSegments(clean string) string {
	segments := strings.Split(clean, ",")
	for i, seg := range segments {
		if isAllDigits(strings.TrimSpace(seg)) {
			return strings.Join(segments[i:], ",")
		}
	}
	return clean
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// scopeToStreetSegment bounds an address (already past
// stripLeadingNonNumericSegments, so it starts at the house-number segment)
// to just that segment plus the very next one, dropping everything after --
// neighborhood, city, township, county, state, zip, country. A plain
// hand-typed address packs the house number, direction, and street name into
// one comma-free segment (e.g. "1801 w 18th st"), so its second segment is
// just the city; a real Nominatim result puts the street in its own second
// segment. Either way, nothing past that second segment is ever needed for
// classification, and leaving it in only risks a false match against
// unrelated address-hierarchy text (see the "South Chicago Township"
// docstring above).
func scopeToStreetSegment(clean string) string {
	segments := strings.SplitN(clean, ",", 3)
	if len(segments) <= 2 {
		return clean
	}
	return segments[0] + "," + segments[1]
}

func isDirEastWestPrefix(clean string) bool {
	// Look for " w " or " e " prefix after house number. Commas are
	// normalized to spaces first, matching detectStreetAxis's own
	// normalization -- without it, a comma glued directly onto the house
	// number (no space) would fold into parts[0] and shift the real
	// direction word out of parts[1].
	parts := strings.Fields(strings.ReplaceAll(clean, ",", " "))
	if len(parts) >= 2 {
		dir := parts[1]
		if dir == "w" || dir == "west" || dir == "e" || dir == "east" {
			return true
		}
	}
	return false
}

// isDirectionWord reports whether tok is a compass-direction word, in either
// its single-letter or spelled-out form (Nominatim always spells these out;
// hand-typed addresses often abbreviate).
func isDirectionWord(tok string) bool {
	return tok == "n" || tok == "s" || tok == "e" || tok == "w" ||
		tok == "north" || tok == "south" || tok == "east" || tok == "west"
}

// matchRegisteredStreetName checks tokens (in order) against
// northSouthNames/eastWestNames, trying the longest registered compound
// name first (up to 3 words, e.g. "ida b wells") down to a single word, so
// a multi-word registered name is never shadowed by a shorter accidental
// match against its own first word or trailing city/state filler.
func matchRegisteredStreetName(tokens []string) (StreetAxis, bool) {
	maxN := 3
	if len(tokens) < maxN {
		maxN = len(tokens)
	}
	for n := maxN; n >= 1; n-- {
		name := strings.Join(tokens[:n], " ")
		if northSouthNames[name] {
			return AxisNorthSouth, true
		}
		if eastWestNames[name] {
			return AxisEastWest, true
		}
	}
	return AxisUnknown, false
}

func detectStreetAxis(clean string) StreetAxis {
	if diagonalNWSERegex.MatchString(clean) {
		return AxisDiagonalNWSE
	}
	if diagonalSWNERegex.MatchString(clean) {
		return AxisDiagonalSWNE
	}

	// Extract street tokens (strip house number and commas)
	parts := strings.Fields(strings.ReplaceAll(clean, ",", " "))
	if len(parts) < 2 {
		return AxisUnknown
	}

	// Skip house number
	tokens := parts[1:]
	// Skip a directional prefix (e.g. "w", "n", "s", "e") -- but only when
	// it actually IS a prefix and not the street's own name. Some Chicago
	// streets are literally named after a compass direction ("North Ave",
	// "Ida B Wells Dr" starting with a direction-shaped word isn't a
	// concern here, but "North Ave" is): when a direction word is
	// immediately followed by a street-type suffix ("North" + "Ave"), it
	// IS the name, not a prefix to discard. Only strip it when a distinct
	// word (the real street name) sits between it and any suffix, e.g.
	// "North Halsted Street" -> "Halsted" is the name, "North" is a
	// prefix.
	if len(tokens) > 1 && isDirectionWord(tokens[0]) && !streetSuffixWords[tokens[1]] {
		tokens = tokens[1:]
	}

	if len(tokens) == 0 {
		return AxisUnknown
	}

	// Check for numbered street (e.g. "17th", "18th", "111th")
	if numberedStreetRegex.MatchString(tokens[0]) {
		return AxisEastWest
	}

	if axis, ok := matchRegisteredStreetName(tokens); ok {
		return axis
	}

	return AxisUnknown
}

// AzimuthToCompassFacing converts an azimuth angle in degrees to the closest 8-wind compass facing.
func AzimuthToCompassFacing(degrees float64) api.UnitMetadataFacing {
	deg := math.Mod(degrees, 360)
	if deg < 0 {
		deg += 360
	}

	switch {
	case deg >= 337.5 || deg < 22.5:
		return api.N
	case deg >= 22.5 && deg < 67.5:
		return api.NE
	case deg >= 67.5 && deg < 112.5:
		return api.E
	case deg >= 112.5 && deg < 157.5:
		return api.SE
	case deg >= 157.5 && deg < 202.5:
		return api.S
	case deg >= 202.5 && deg < 247.5:
		return api.SW
	case deg >= 247.5 && deg < 292.5:
		return api.W
	case deg >= 292.5 && deg < 337.5:
		return api.NW
	default:
		return api.N
	}
}
