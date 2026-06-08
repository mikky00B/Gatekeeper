package logger

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goproxy/internal/router"
)

func TestMiddlewareWritesRequestEntry(t *testing.T) {
	sink := NewChannelSink(1)
	handler := Middleware(sink)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://gateway.test/api/users", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	select {
	case entry := <-sink.Entries():
		if entry.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", entry.Method)
		}
		if entry.Path != "/api/users" {
			t.Fatalf("path = %q, want /api/users", entry.Path)
		}
		if entry.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", entry.StatusCode, http.StatusCreated)
		}
		if entry.RemoteAddr != "192.0.2.10:1234" {
			t.Fatalf("remote addr = %q, want 192.0.2.10:1234", entry.RemoteAddr)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for log entry")
	}
}

func TestMiddlewareCapturesRouteID(t *testing.T) {
	sink := NewChannelSink(1)
	r := router.New([]router.Route{
		{
			ID:         "api",
			PathPrefix: "/api",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})
	handler := Middleware(sink)(r)

	req := httptest.NewRequest(http.MethodGet, "http://gateway.test/api/users", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	select {
	case entry := <-sink.Entries():
		if entry.RouteID != "api" {
			t.Fatalf("route id = %q, want api", entry.RouteID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for log entry")
	}
}

func TestChannelSinkDropsWhenFull(t *testing.T) {
	sink := NewChannelSink(1)
	sink.Write(Entry{Path: "/first"})
	sink.Write(Entry{Path: "/second"})

	entry := <-sink.Entries()
	if entry.Path != "/first" {
		t.Fatalf("path = %q, want /first", entry.Path)
	}
}
