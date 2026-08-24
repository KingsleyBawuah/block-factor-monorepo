package spatial_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/KingsleyBawuah/block-factor-monorepo/services/api/internal/spatial"
)

type mockQueryRow struct {
	scanFunc func(dest ...any) error
}

func (m *mockQueryRow) Scan(dest ...any) error {
	return m.scanFunc(dest...)
}

type mockDB struct {
	capturedSQL  string
	capturedArgs []any
	scanResult   bool
	scanErr      error
}

func (m *mockDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	m.capturedSQL = sql
	m.capturedArgs = args
	return &mockQueryRow{
		scanFunc: func(dest ...any) error {
			if m.scanErr != nil {
				return m.scanErr
			}
			if len(dest) > 0 {
				if ptr, ok := dest[0].(*bool); ok {
					*ptr = m.scanResult
				}
			}
			return nil
		},
	}
}

func TestIsWithinCityBoundary_SuccessCovered(t *testing.T) {
	db := &mockDB{scanResult: true}
	covered, err := spatial.IsWithinCityBoundary(context.Background(), db, 41.8781, -87.6298)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !covered {
		t.Errorf("expected covered=true, got false")
	}
	if len(db.capturedArgs) != 2 || db.capturedArgs[0] != -87.6298 || db.capturedArgs[1] != 41.8781 {
		t.Errorf("expected args [lon, lat], got %v", db.capturedArgs)
	}
}

func TestIsWithinCityBoundary_SuccessOutside(t *testing.T) {
	db := &mockDB{scanResult: false}
	covered, err := spatial.IsWithinCityBoundary(context.Background(), db, 42.1000, -88.0000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if covered {
		t.Errorf("expected covered=false, got true")
	}
}

func TestIsWithinCityBoundary_EmptyTable(t *testing.T) {
	db := &mockDB{scanErr: pgx.ErrNoRows}
	_, err := spatial.IsWithinCityBoundary(context.Background(), db, 41.8781, -87.6298)
	if err == nil {
		t.Fatal("expected error for empty table, got nil")
	}
}

func TestIsWithinCityBoundary_QueryError(t *testing.T) {
	db := &mockDB{scanErr: errors.New("connection failed")}
	_, err := spatial.IsWithinCityBoundary(context.Background(), db, 41.8781, -87.6298)
	if err == nil {
		t.Fatal("expected error on query failure, got nil")
	}
}

func TestIsWithinCityBoundary_NilDB(t *testing.T) {
	_, err := spatial.IsWithinCityBoundary(context.Background(), nil, 41.8781, -87.6298)
	if err == nil {
		t.Fatal("expected error on nil db, got nil")
	}
}
