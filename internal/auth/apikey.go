package auth

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
)

type contextKey string

const principalContextKey contextKey = "principal"

type APIKey struct {
	ID     string
	Key    string
	Tenant string
}

type Principal struct {
	KeyID  string
	Tenant string
}

type APIKeyStore interface {
	Lookup(key string) (APIKey, bool)
}

type MultiAPIKeyStore []APIKeyStore

func (s MultiAPIKeyStore) Lookup(key string) (APIKey, bool) {
	for _, store := range s {
		if store == nil {
			continue
		}
		if found, ok := store.Lookup(key); ok {
			return found, true
		}
	}
	return APIKey{}, false
}

type StaticAPIKeyStore struct {
	keys []APIKey
}

func NewStaticAPIKeyStore(keys []APIKey) *StaticAPIKeyStore {
	copied := make([]APIKey, 0, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(key.Key) == "" {
			continue
		}
		copied = append(copied, key)
	}

	return &StaticAPIKeyStore{keys: copied}
}

func (s *StaticAPIKeyStore) Lookup(candidate string) (APIKey, bool) {
	for _, key := range s.keys {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(key.Key)) == 1 {
			return key, true
		}
	}
	return APIKey{}, false
}

func APIKeyMiddleware(store APIKeyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey := strings.TrimSpace(r.Header.Get("X-API-Key"))
			if rawKey == "" {
				writeUnauthorized(w)
				return
			}

			key, ok := store.Lookup(rawKey)
			if !ok {
				writeUnauthorized(w)
				return
			}

			principal := Principal{
				KeyID:  key.ID,
				Tenant: key.Tenant,
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalContextKey, principal)))
		})
	}
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey).(Principal)
	return principal, ok
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `ApiKey realm="gateway"`)
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}
