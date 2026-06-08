package analytics

import (
	"sort"
	"sync"
	"time"

	"goproxy/internal/logger"
)

type RouteMetrics struct {
	RouteID      string        `json:"route_id"`
	RequestCount int64         `json:"request_count"`
	ErrorCount   int64         `json:"error_count"`
	ErrorRate    float64       `json:"error_rate"`
	P50Latency   time.Duration `json:"p50_latency"`
	P95Latency   time.Duration `json:"p95_latency"`
}

type Aggregator struct {
	mu     sync.RWMutex
	routes map[string][]logger.Entry
	limit  int
}

func NewAggregator(limit int) *Aggregator {
	if limit < 1 {
		limit = 10000
	}
	return &Aggregator{
		routes: make(map[string][]logger.Entry),
		limit:  limit,
	}
}

func (a *Aggregator) Write(entry logger.Entry) {
	if a == nil {
		return
	}
	routeID := entry.RouteID
	if routeID == "" {
		routeID = "unmatched"
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	entries := append(a.routes[routeID], entry)
	if len(entries) > a.limit {
		copy(entries, entries[len(entries)-a.limit:])
		entries = entries[:a.limit]
	}
	a.routes[routeID] = entries
}

func (a *Aggregator) Snapshot() []RouteMetrics {
	if a == nil {
		return nil
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	metrics := make([]RouteMetrics, 0, len(a.routes))
	for routeID, entries := range a.routes {
		if len(entries) == 0 {
			continue
		}

		durations := make([]time.Duration, 0, len(entries))
		var errors int64
		for _, entry := range entries {
			durations = append(durations, entry.Duration)
			if entry.StatusCode >= 500 {
				errors++
			}
		}
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

		count := int64(len(entries))
		metrics = append(metrics, RouteMetrics{
			RouteID:      routeID,
			RequestCount: count,
			ErrorCount:   errors,
			ErrorRate:    float64(errors) / float64(count),
			P50Latency:   percentile(durations, 0.50),
			P95Latency:   percentile(durations, 0.95),
		})
	}

	sort.Slice(metrics, func(i, j int) bool { return metrics[i].RouteID < metrics[j].RouteID })
	return metrics
}

func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	if p <= 0 {
		return values[0]
	}
	if p >= 1 {
		return values[len(values)-1]
	}

	index := int(float64(len(values)-1) * p)
	return values[index]
}
