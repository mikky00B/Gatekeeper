package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterChoosesLongestMatchingPrefix(t *testing.T) {
	r := New([]Route{
		{
			ID:         "root",
			PathPrefix: "/",
			Handler:    writeHandler("root"),
		},
		{
			ID:         "api",
			PathPrefix: "/api",
			Handler:    writeHandler("api"),
		},
	})

	req := httptest.NewRequest(http.MethodGet, "http://gateway.test/api/users", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Body.String() != "api" {
		t.Fatalf("body = %q, want api", rec.Body.String())
	}
}

func TestRouterHonorsMethods(t *testing.T) {
	r := New([]Route{
		{
			ID:         "api",
			PathPrefix: "/api",
			Methods:    []string{http.MethodPost},
			Handler:    writeHandler("api"),
		},
	})

	req := httptest.NewRequest(http.MethodGet, "http://gateway.test/api/users", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRouterCanStripPrefix(t *testing.T) {
	var gotPath string
	r := New([]Route{
		{
			ID:          "api",
			PathPrefix:  "/api",
			StripPrefix: true,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				gotPath = req.URL.Path
			}),
		},
	})

	req := httptest.NewRequest(http.MethodGet, "http://gateway.test/api/users", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if gotPath != "/users" {
		t.Fatalf("path = %q, want /users", gotPath)
	}
}

func writeHandler(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte(body))
	})
}
