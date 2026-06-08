package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type PostgresStore struct {
	db Queryer
}

func NewPostgresStore(db Queryer) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) RouteMetrics(ctx context.Context, since time.Time) ([]RouteMetrics, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("analytics store requires a database")
	}

	const query = `
SELECT route_id,
       COUNT(*) AS request_count,
       COUNT(*) FILTER (WHERE status_code >= 500) AS error_count,
       COALESCE(PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY duration_ms), 0) AS p50_ms,
       COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms), 0) AS p95_ms
FROM request_logs
WHERE created_at >= $1
GROUP BY route_id
ORDER BY route_id`

	rows, err := s.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, fmt.Errorf("query route metrics: %w", err)
	}
	defer rows.Close()

	var metrics []RouteMetrics
	for rows.Next() {
		var item RouteMetrics
		var p50MS, p95MS float64
		if err := rows.Scan(&item.RouteID, &item.RequestCount, &item.ErrorCount, &p50MS, &p95MS); err != nil {
			return nil, fmt.Errorf("scan route metrics: %w", err)
		}
		if item.RequestCount > 0 {
			item.ErrorRate = float64(item.ErrorCount) / float64(item.RequestCount)
		}
		item.P50Latency = time.Duration(p50MS) * time.Millisecond
		item.P95Latency = time.Duration(p95MS) * time.Millisecond
		metrics = append(metrics, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read route metrics: %w", err)
	}

	return metrics, nil
}
