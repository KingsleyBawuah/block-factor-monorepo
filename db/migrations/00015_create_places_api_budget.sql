-- +goose Up
-- +goose StatementBegin
-- Daily spend guard for the Google Places API (PRD 5.4 / ARCHITECTURE 5.4):
-- one row per calendar day, incremented atomically on every call attempt via
-- a single UPSERT (internal/scoring.DBPlacesBudget) so concurrent requests
-- can't race past the cap. Deliberately a Postgres table rather than an
-- in-process counter: an in-memory counter resets to zero on every
-- restart/redeploy, silently reopening the day's cap right when a
-- restart-triggered retry storm makes that most dangerous. A Postgres row
-- survives restarts and is shared correctly across multiple server
-- instances. One row per day is negligible growth (365/year); no cleanup
-- job needed.
CREATE TABLE places_api_budget (
    budget_date DATE PRIMARY KEY,
    call_count INTEGER NOT NULL DEFAULT 0 CHECK (call_count >= 0)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS places_api_budget;
-- +goose StatementEnd
