package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyForwardsRequestsToTarget(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/users" {
			t.Fatalf("path = %q, want /v1/users", r.URL.Path)
		}
		if r.URL.Scheme != "http" {
			t.Fatalf("scheme = %q, want http", r.URL.Scheme)
		}
		if r.URL.Host != "upstream.test" {
			t.Fatalf("host = %q, want upstream.test", r.URL.Host)
		}
		if got := r.Header.Get("X-Api-Key"); got != "" {
			t.Fatalf("X-Api-Key was forwarded: %q", got)
		}
		if got := r.Header.Get("X-Forwarded-Host"); got != "gateway.test" {
			t.Fatalf("X-Forwarded-Host = %q, want gateway.test", got)
		}

		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       io.NopCloser(strings.NewReader("proxied")),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	})

	p, err := NewWithTransport("http://upstream.test", transport)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://gateway.test/v1/users", nil)
	req.Header.Set("X-Api-Key", "secret")
	rec := httptest.NewRecorder()

	p.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusAccepted)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "proxied" {
		t.Fatalf("body = %q, want proxied", string(body))
	}
}

func TestNewRejectsInvalidTarget(t *testing.T) {
	if _, err := New("localhost:3000"); err == nil {
		t.Fatal("New returned nil error for target without scheme")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}
