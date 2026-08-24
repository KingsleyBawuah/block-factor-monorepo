-- +goose Up
-- +goose StatementBegin
-- Powers S_transit's rail service-quality bonus (PRD 5.2). One row per CTA
-- 'L' line (line_name matches cta_stops.lines values exactly: Red, Blue,
-- Brown, Green, Orange, Pink, Purple, Yellow -- enforced below), derived
-- from the same CTA static GTFS feed already used for bus_corridors
-- (route_type = 1 rather than 3). The AM-peak pass reuses that
-- derivation's weekday-only service filter as-is (PRD 5.2 defines AM peak
-- as weekday 7-9 AM). The overnight pass reuses its GTFS after-midnight
-- (hour >= 24) time parsing, but NOT its weekday-only service filter --
-- PRD 5.2 defines the overnight window as "1:00-4:00 AM, any day", so
-- that pass needs its own any-day service filter rather than the
-- Mon-Fri-only one, or it would silently miss weekend-only overnight
-- service.
--
-- No geom column and no GIST index -- headway is a property of the line,
-- not a place, so there's nothing spatial to store here.
--
-- overnight_headway_minutes is nullable rather than sentinel-filled (unlike
-- am_peak_headway_minutes, which reuses bus_corridors' 999.00 no-service
-- sentinel): "no scheduled overnight service at all" is a real, expected
-- state for most lines (confirmed directly against live GTFS stop_times --
-- e.g. the Yellow Line has zero stop_times rows in the 1-4 AM window,
-- versus thousands for Red), not a rare edge case, so scoring should see
-- an explicit NULL rather than do arithmetic against a magic number.
--
-- Deliberately skips the staging-table + atomic-swap pattern used by
-- bus_corridors/safety_grid_percentiles: those exist to avoid holding a
-- long lock while computing thousands of rows with expensive geometry
-- work. This table is 8 rows with no geometry, so a single
-- BEGIN; TRUNCATE; INSERT; COMMIT already has an effectively-zero lock
-- duration -- the staging-table ceremony would be pure overhead here.
--
-- Full hours-of-operation (for a future Phase 5 "Runs 4:30 AM - 1:15 AM"
-- UI badge) is deliberately out of scope for this table -- see PRD 5.2
-- "Future Consideration -- Line Schedule Display".
CREATE TABLE cta_line_service (
    line_name TEXT PRIMARY KEY CHECK (
        line_name IN ('Red', 'Blue', 'Brown', 'Green', 'Orange', 'Pink', 'Purple', 'Yellow')
    ),
    am_peak_headway_minutes NUMERIC(5, 2) NOT NULL CHECK (am_peak_headway_minutes > 0),
    overnight_headway_minutes NUMERIC(5, 2) CHECK (overnight_headway_minutes IS NULL OR overnight_headway_minutes > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS cta_line_service;
-- +goose StatementEnd
