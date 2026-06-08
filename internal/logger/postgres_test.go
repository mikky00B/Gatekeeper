package logger

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestPostgresWriterFlushInsertsBatch(t *testing.T) {
	db := &fakeExecer{}
	writer := NewPostgresWriter(db, 100, time.Second)

	entries := []Entry{
		{
			Method:     "GET",
			Path:       "/api/users",
			RouteID:    "api",
			StatusCode: 200,
			Duration:   150 * time.Millisecond,
			RemoteAddr: "192.0.2.1:1000",
			Timestamp:  time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
		},
		{
			Method:     "POST",
			Path:       "/api/users",
			RouteID:    "api",
			StatusCode: 201,
			Duration:   275 * time.Millisecond,
			RemoteAddr: "192.0.2.2:1000",
			Timestamp:  time.Date(2026, 5, 29, 10, 1, 0, 0, time.UTC),
		},
	}

	if err := writer.Flush(context.Background(), entries); err != nil {
		t.Fatalf("Flush returned error: %v", err)
	}

	if !strings.Contains(db.query, "INSERT INTO request_logs") {
		t.Fatalf("query = %q, want request_logs insert", db.query)
	}
	if !strings.Contains(db.query, "($1, $2, $3, $4, $5, $6, $7), ($8, $9, $10, $11, $12, $13, $14)") {
		t.Fatalf("query placeholders = %q", db.query)
	}
	if len(db.args) != 14 {
		t.Fatalf("args length = %d, want 14", len(db.args))
	}
	if db.args[0] != "GET" {
		t.Fatalf("first method arg = %v, want GET", db.args[0])
	}
	if db.args[2] != "api" {
		t.Fatalf("first route id arg = %v, want api", db.args[2])
	}
	if db.args[4] != int64(150) {
		t.Fatalf("first duration arg = %v, want 150", db.args[4])
	}
}

func TestPostgresWriterFlushNoopsEmptyBatch(t *testing.T) {
	db := &fakeExecer{}
	writer := NewPostgresWriter(db, 100, time.Second)

	if err := writer.Flush(context.Background(), nil); err != nil {
		t.Fatalf("Flush returned error: %v", err)
	}
	if db.query != "" {
		t.Fatalf("query = %q, want empty", db.query)
	}
}

func TestPostgresWriterRunFlushesOnClosedChannel(t *testing.T) {
	db := &fakeExecer{}
	writer := NewPostgresWriter(db, 10, time.Hour)
	entries := make(chan Entry, 1)
	entries <- Entry{
		Method:     "GET",
		Path:       "/healthz",
		StatusCode: 200,
		Timestamp:  time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
	}
	close(entries)

	if err := writer.Run(context.Background(), entries); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(db.args) != 7 {
		t.Fatalf("args length = %d, want 7", len(db.args))
	}
}

type fakeExecer struct {
	query string
	args  []any
}

func (f *fakeExecer) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	f.query = query
	f.args = args
	return fakeResult(0), nil
}

type fakeResult int64

func (f fakeResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (f fakeResult) RowsAffected() (int64, error) {
	return int64(f), nil
}
