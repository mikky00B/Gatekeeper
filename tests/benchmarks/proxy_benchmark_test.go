package benchmarks

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goproxy/internal/proxy"
)

func BenchmarkProxy(b *testing.B) {
	p, err := proxy.NewWithTransport("http://upstream.test", roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	}))
	if err != nil {
		b.Fatalf("proxy.New returned error: %v", err)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://gateway.test/resource", nil)
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			b.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
