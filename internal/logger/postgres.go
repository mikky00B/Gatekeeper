package logger

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type PostgresWriter struct {
	db        Execer
	batchSize int
	interval  time.Duration
}

func NewPostgresWriter(db Execer, batchSize int, interval time.Duration) *PostgresWriter {
	if batchSize < 1 {
		batchSize = 100
	}
	if interval <= 0 {
		interval = time.Second
	}

	return &PostgresWriter{
		db:        db,
		batchSize: batchSize,
		interval:  interval,
	}
}

func (w *PostgresWriter) Run(ctx context.Context, entries <-chan Entry) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	batch := make([]Entry, 0, w.batchSize)
	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				if err := w.Flush(context.Background(), batch); err != nil {
					return err
				}
			}
			return ctx.Err()
		case entry, ok := <-entries:
			if !ok {
				if len(batch) > 0 {
					return w.Flush(ctx, batch)
				}
				return nil
			}
			batch = append(batch, entry)
			if len(batch) >= w.batchSize {
				if err := w.Flush(ctx, batch); err != nil {
					return err
				}
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) == 0 {
				continue
			}
			if err := w.Flush(ctx, batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
}

func (w *PostgresWriter) Flush(ctx context.Context, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}
	if w.db == nil {
		return fmt.Errorf("postgres writer requires a database")
	}

	var query strings.Builder
	query.WriteString("INSERT INTO request_logs ")
	query.WriteString("(method, path, route_id, status_code, duration_ms, remote_addr, created_at) VALUES ")

	args := make([]any, 0, len(entries)*7)
	for i, entry := range entries {
		if i > 0 {
			query.WriteString(", ")
		}

		base := i*7 + 1
		query.WriteString(fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)", base, base+1, base+2, base+3, base+4, base+5, base+6))
		args = append(args,
			entry.Method,
			entry.Path,
			entry.RouteID,
			entry.StatusCode,
			entry.Duration.Milliseconds(),
			entry.RemoteAddr,
			entry.Timestamp,
		)
	}

	_, err := w.db.ExecContext(ctx, query.String(), args...)
	if err != nil {
		return fmt.Errorf("insert request logs: %w", err)
	}

	return nil
}
