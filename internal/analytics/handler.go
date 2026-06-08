package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Handler struct {
	aggregator *Aggregator
	store      Store
	window     time.Duration
}

func NewHandler(aggregator *Aggregator) *Handler {
	return &Handler{aggregator: aggregator}
}

type Store interface {
	RouteMetrics(ctx context.Context, since time.Time) ([]RouteMetrics, error)
}

func NewPersistentHandler(store Store, fallback *Aggregator, window time.Duration) *Handler {
	if window <= 0 {
		window = time.Hour
	}
	return &Handler{
		aggregator: fallback,
		store:      store,
		window:     window,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/analytics/routes" {
		http.NotFound(w, r)
		return
	}

	type routeMetrics struct {
		RouteID      string  `json:"route_id"`
		RequestCount int64   `json:"request_count"`
		ErrorCount   int64   `json:"error_count"`
		ErrorRate    float64 `json:"error_rate"`
		P50LatencyMS int64   `json:"p50_latency_ms"`
		P95LatencyMS int64   `json:"p95_latency_ms"`
	}

	snapshot, err := h.snapshot(r.Context())
	if err != nil {
		http.Error(w, "analytics unavailable", http.StatusServiceUnavailable)
		return
	}
	response := make([]routeMetrics, 0, len(snapshot))
	for _, item := range snapshot {
		response = append(response, routeMetrics{
			RouteID:      item.RouteID,
			RequestCount: item.RequestCount,
			ErrorCount:   item.ErrorCount,
			ErrorRate:    item.ErrorRate,
			P50LatencyMS: item.P50Latency.Milliseconds(),
			P95LatencyMS: item.P95Latency.Milliseconds(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (h *Handler) snapshot(ctx context.Context) ([]RouteMetrics, error) {
	if h != nil && h.store != nil {
		return h.store.RouteMetrics(ctx, time.Now().Add(-h.window))
	}
	if h == nil || h.aggregator == nil {
		return nil, nil
	}
	return h.aggregator.Snapshot(), nil
}
