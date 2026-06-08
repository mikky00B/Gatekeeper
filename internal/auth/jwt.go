package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const jwtPrincipalContextKey contextKey = "jwt_principal"

type JWTVerifier struct {
	Secret   []byte
	Issuer   string
	Audience string
	Now      func() time.Time
}

type JWTPrincipal struct {
	Subject string
	Tenant  string
	Claims  map[string]any
}

func JWTMiddleware(verifier JWTVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r.Header.Get("Authorization"))
			if token == "" {
				writeBearerUnauthorized(w)
				return
			}

			principal, err := verifier.Verify(token)
			if err != nil {
				writeBearerUnauthorized(w)
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), jwtPrincipalContextKey, principal)))
		})
	}
}

func (v JWTVerifier) Verify(token string) (JWTPrincipal, error) {
	if len(v.Secret) == 0 {
		return JWTPrincipal{}, errors.New("jwt secret is required")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return JWTPrincipal{}, errors.New("jwt must have three parts")
	}

	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return JWTPrincipal{}, err
	}
	mac := hmac.New(sha256.New, v.Secret)
	_, _ = mac.Write([]byte(signingInput))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return JWTPrincipal{}, errors.New("jwt signature mismatch")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return JWTPrincipal{}, err
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return JWTPrincipal{}, err
	}
	if header.Algorithm != "HS256" {
		return JWTPrincipal{}, errors.New("only HS256 jwt tokens are supported")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return JWTPrincipal{}, err
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return JWTPrincipal{}, err
	}

	now := time.Now
	if v.Now != nil {
		now = v.Now
	}
	if exp, ok := numberClaim(claims, "exp"); ok && now().Unix() >= int64(exp) {
		return JWTPrincipal{}, errors.New("jwt expired")
	}
	if nbf, ok := numberClaim(claims, "nbf"); ok && now().Unix() < int64(nbf) {
		return JWTPrincipal{}, errors.New("jwt not active yet")
	}
	if v.Issuer != "" && stringClaim(claims, "iss") != v.Issuer {
		return JWTPrincipal{}, errors.New("jwt issuer mismatch")
	}
	if v.Audience != "" && !audienceMatches(claims["aud"], v.Audience) {
		return JWTPrincipal{}, errors.New("jwt audience mismatch")
	}

	return JWTPrincipal{
		Subject: stringClaim(claims, "sub"),
		Tenant:  stringClaim(claims, "tenant"),
		Claims:  claims,
	}, nil
}

func JWTPrincipalFromContext(ctx context.Context) (JWTPrincipal, bool) {
	principal, ok := ctx.Value(jwtPrincipalContextKey).(JWTPrincipal)
	return principal, ok
}

func bearerToken(value string) string {
	scheme, token, ok := strings.Cut(strings.TrimSpace(value), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func writeBearerUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="gateway"`)
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

func stringClaim(claims map[string]any, key string) string {
	value, _ := claims[key].(string)
	return value
}

func numberClaim(claims map[string]any, key string) (float64, bool) {
	switch value := claims[key].(type) {
	case float64:
		return value, true
	case int64:
		return float64(value), true
	default:
		return 0, false
	}
}

func audienceMatches(value any, expected string) bool {
	switch audience := value.(type) {
	case string:
		return audience == expected
	case []any:
		for _, item := range audience {
			if text, ok := item.(string); ok && text == expected {
				return true
			}
		}
	}
	return false
}
