# BlockFactor: GIS & Spatial Concepts Glossary

**Status:** Active Reference — grows incrementally
**See also:**
* [ARCHITECTURE.md](./ARCHITECTURE.md) — where most of these concepts are actually used
* [AGENTS.md](../AGENTS.md) — Core Philosophy §1 (why this doc exists)

---

## Overview

This is a plain-English glossary of the GIS (Geographic Information Systems) and spatial-data terms that come up while building BlockFactor — written for a software engineer with no prior GIS background. Entries favor a short analogy and a concrete tie-in to how BlockFactor actually uses the concept over a formal textbook definition. New terms get added here as they come up in conversation, not written all at once — if a term is unfamiliar and isn't listed yet, ask and it'll be added.

---

## Bounding Box

The smallest rectangle that fully contains a shape, described by its min/max latitude and longitude edges — like the rectangular crop tool in an image editor, snapped tight around whatever you're framing. BlockFactor uses this idea for the 400m spatial grid cache: Google Places results are cached per grid cell (a small bounding box) rather than per exact address, so many nearby addresses share the same cached data instead of each triggering a fresh paid API call.

## Coordinate Reference System (CRS)

A CRS defines how a flat pair of numbers (like `41.8781, -87.6298`) maps onto an actual physical location on Earth. It's the difference between "3 inches from the top-left corner of this piece of paper" and "this exact spot in the real world" — the CRS is the agreed-upon rulebook that makes coordinates meaningful and comparable. BlockFactor uses one CRS throughout: WGS84 (see below), identified by its SRID `4326`.

## GDAL / ogr2ogr

GDAL (Geospatial Data Abstraction Library) is the standard open-source toolkit for reading and converting between spatial file formats — think of it as `ffmpeg`, but for geospatial data instead of video. `ogr2ogr` is its command-line format-converter: point it at an input (a shapefile, a GeoPackage, a FileGDB) and an output (PostGIS, GeoJSON, another shapefile), and it translates one into the other, optionally reprojecting coordinates or filtering features along the way. BlockFactor's Phase 2 datasets that ship as shapefiles (bike routes, parks — see `DATA_SOURCES.md`) are the reason this term came up: shelling out to `ogr2ogr` to load them straight into PostGIS is one of the two options being weighed for the ARCHITECTURE.md §13.2 ingestion-tooling decision, against parsing the shapefile format directly in Go.

## Geocoding

Turning a human-readable address ("233 S Wacker Dr, Chicago, IL") into a coordinate pair (`41.8789, -87.6359`) a computer can do math with. Reverse geocoding goes the other direction — coordinate back to address. BlockFactor geocodes once per request (via Nominatim) and does all scoring math on the resulting coordinates — the raw address string is never persisted (see `PRIVACY_AND_DATA_HANDLING.md`).

## Geography vs. Geometry (PostGIS column types)

PostGIS offers two ways to store spatial data. `geometry` treats coordinates as flat, graph-paper math — fast, but distances distort the further you are from the equator unless you handle projection manually. `geography` treats coordinates as points on the Earth's actual curved surface — slightly more computation, but distances in meters come out correct everywhere with no manual projection math. BlockFactor uses `geography(Point, 4326)` throughout specifically so "how many meters to the nearest supermarket" just works correctly out of the box.

## Geohash

A way of encoding a location into a short string, where each added character narrows the location further — like a zoom level carved directly into the string. BlockFactor logs a truncated geohash instead of a raw address: it tells you roughly which neighborhood a request came from (useful for debugging) without ever revealing or storing the exact address.

## GiST Index

GiST (Generalized Search Tree) is a database index built for spatial (and other non-standard) data, where a normal index doesn't help. A normal index is like alphabetical order in a phone book — great for "find this exact name," useless for "find everyone within 2 miles of this point." A GiST index organizes data by spatial containment/overlap instead, so PostGIS can instantly narrow down "which of 300,000 crime records are anywhere near this address" rather than checking every row. This is what makes BlockFactor's `ST_DWithin` radius queries run in milliseconds instead of seconds.

## GTFS (General Transit Feed Specification)

The standard file format transit agencies (like the CTA) publish schedules in — stops, routes, trip times, service frequency. It's the de facto universal format every trip-planning tool (Google Maps, OpenTripPlanner, etc.) knows how to read. BlockFactor ingests CTA's GTFS feed both to build the OpenTripPlanner routing graph and to derive which bus stops qualify as "high-frequency" (see Headway).

## Headway

Transit jargon for "how long you wait between vehicles" — a bus with a 10-minute headway shows up roughly every 10 minutes. It's the transit-scheduling equivalent of a traffic light's cycle time. BlockFactor computes headway from GTFS data to decide whether a bus stop is frequent enough to boost a property's transit score.

## Invalid Geometry (Self-Intersection)

A polygon is only "valid" if its boundary is a clean, non-crossing loop — like a rubber band laid flat with no twists. A self-intersecting polygon is what happens when that boundary line crosses back over itself somewhere, like the rubber band got a twist in it and formed a little figure-eight pinch instead of a simple loop. At that crossing point, "inside" and "outside" the shape become ambiguous, which breaks spatial math like computing area or checking whether a point falls inside it. It's a common real-world defect in government GIS data — usually from boundaries digitized by hand over decades, especially for large or complex shapes where a point was traced slightly out of order or two merged boundary pieces don't line up exactly. `ST_MakeValid()` is PostGIS's repair function for this. BlockFactor hit this for real: 11 of Chicago's 617 park boundaries — including Washington Park, Grant Park, and Lincoln Park — have this defect in the source data, fixed at ingestion time (see `docs/DATA_SOURCES.md` "Parks").

## Isochrone

A boundary drawn on a map showing everywhere reachable within a fixed amount of time from a starting point (from "iso" = same, "chrone" = time). Picture a stone dropped in a pond — the ripple's edge at any given second shows "how far the wave has traveled by now." An isochrone is that same idea applied to travel, but it follows the real street/transit network rather than a straight-line radius: it bulges out along a fast route (like next to a train line) and pinches in where movement is blocked or slow (a river with no bridge). BlockFactor renders a 15-minute walk isochrone and a 30-minute transit isochrone per address as shaded polygons on the map.

## PBF (Protocolbuffer Binary Format)

A compact binary encoding OpenStreetMap uses for its data extracts, as an alternative to the older plain-text `.osm` XML format — same nodes/ways/relations and tags, just packed far more densely, so a regional extract is a fraction of the size and much faster to parse. Think of it as the difference between a verbose XML export and a zipped binary log of the same records. BlockFactor ingests a Chicago-area `.osm.pbf` extract (from a provider like Geofabrik) to pull out POIs (points of interest) — grocery stores, pharmacies, etc. — identified by their OSM tags (`shop=supermarket`, `amenity=pharmacy`).

## Polygon

A shape made from a closed loop of straight lines: connect a series of coordinate points one after another, then draw one final line back to where you started, and whatever's enclosed is the polygon. It's how GIS represents anything with an *area* — a park, a lake, a city boundary — as opposed to a single location (a Point) or a path (a LineString). A MultiPolygon is just a polygon record allowed to be more than one separate piece at once: BlockFactor's own `city_boundary` is technically a MultiPolygon because O'Hare Airport sits detached from the rest of Chicago, making "the city" two disconnected pieces stored as one record. BlockFactor stores parks, flood zones, and the city boundary as (Multi)Polygons — distinct from points like CTA stations or lines like bike routes.

## ST_DWithin

A PostGIS spatial query function that answers "is this thing within X meters of that thing?" efficiently, using a GiST index rather than brute-force checking every record. It's the core building block behind nearly every BlockFactor scoring factor — "is there a supermarket within 0.3 miles," "is there a CTA stop within 0.25 miles" — all of it boils down to an `ST_DWithin` call under the hood.

## SRID (Spatial Reference Identifier)

A numeric ID that names a specific coordinate reference system — similar to how a country code (+1, +44) identifies which phone numbering system you're using. BlockFactor uses SRID `4326`, which specifically means WGS84 (see below). Every spatial column in the database is tagged `4326` so nothing gets mixed up with a different, incompatible projection.

## WGS84

The specific real-world reference system that defines what "latitude 41.8781, longitude -87.6298" actually means — the standard GPS satellites and virtually all consumer mapping software use. When people say "regular GPS coordinates," they mean WGS84. It's what SRID `4326` refers to.
