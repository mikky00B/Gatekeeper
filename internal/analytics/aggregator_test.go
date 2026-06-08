package analytics

import (
	"testing"
	"time"

	"goproxy/internal/logger"
)

func TestAggregatorSnapshotsRouteMetrics(t *testing.T) {
	aggregator := NewAggregator(10)
	aggregator.Write(logger.Entry{RouteID: "api", StatusCode: 200, Duration: 10 * time.Millisecond})
	aggregator.Write(logger.Entry{RouteID: "api", StatusCode: 502, Duration: 30 * time.Millisecond})

	snapshot := aggregator.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("snapshot length = %d, want 1", len(snapshot))
	}
	if snapshot[0].RouteID != "api" {
		t.Fatalf("route id = %q, want api", snapshot[0].RouteID)
	}
	if snapshot[0].RequestCount != 2 {
		t.Fatalf("request count = %d, want 2", snapshot[0].RequestCount)
	}
	if snapshot[0].ErrorCount != 1 {
		t.Fatalf("error count = %d, want 1", snapshot[0].ErrorCount)
	}
	if snapshot[0].ErrorRate != 0.5 {
		t.Fatalf("error rate = %f, want 0.5", snapshot[0].ErrorRate)
	}
}
