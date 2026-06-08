package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goproxy/internal/auth"
	"goproxy/internal/proxy"
	"goproxy/internal/ratelimit"
	"goproxy/internal/router"
)

func TestAuthenticatedRateLimitedProxyFlow(t *testing.T) {
	p, err := proxy.NewWithTransport("http://upstream.test", roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/users" {
			t.Fatalf("upstream path = %q, want /users", r.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	}))
	if err != nil {
		t.Fatalf("proxy.New returned error: %v", err)
	}

	handler := router.New([]router.Route{
		{
			ID:          "api",
			PathPrefix:  "/api",
			StripPrefix: true,
			Handler: proxy.Chain(
				p,
				auth.APIKeyMiddleware(auth.NewStaticAPIKeyStore([]auth.APIKey{{ID: "dev", Key: "secret", Tenant: "local"}})),
				ratelimit.Middleware(ratelimit.NewLimiter(1, 1), ratelimit.APIKeyOrIPKey),
			),
		},
	})

	req := httptest.NewRequest(http.MethodGet, "http://gateway.test/api/users", nil)
	req.Header.Set("X-API-Key", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
