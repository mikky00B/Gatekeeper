package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIKeyMiddlewareRejectsMissingKey(t *testing.T) {
	store := NewStaticAPIKeyStore([]APIKey{{ID: "test", Key: "secret", Tenant: "tenant-a"}})
	handler := APIKeyMiddleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run")
	}))

	req := httptest.NewRequest(http.MethodGet, "http://gateway.test/private", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAPIKeyMiddlewareRejectsInvalidKey(t *testing.T) {
	store := NewStaticAPIKeyStore([]APIKey{{ID: "test", Key: "secret", Tenant: "tenant-a"}})
	handler := APIKeyMiddleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run")
	}))

	req := httptest.NewRequest(http.MethodGet, "http://gateway.test/private", nil)
	req.Header.Set("X-API-Key", "wrong")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAPIKeyMiddlewareAddsPrincipal(t *testing.T) {
	store := NewStaticAPIKeyStore([]APIKey{{ID: "test", Key: "secret", Tenant: "tenant-a"}})
	handler := APIKeyMiddleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			t.Fatal("principal missing from context")
		}
		if principal.KeyID != "test" {
			t.Fatalf("principal key id = %q, want test", principal.KeyID)
		}
		if principal.Tenant != "tenant-a" {
			t.Fatalf("principal tenant = %q, want tenant-a", principal.Tenant)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://gateway.test/private", nil)
	req.Header.Set("X-API-Key", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
