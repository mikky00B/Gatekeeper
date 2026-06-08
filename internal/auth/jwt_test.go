package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"
)

func TestJWTVerifierAcceptsValidHS256Token(t *testing.T) {
	token := signJWT(t, `{"alg":"HS256","typ":"JWT"}`, `{"sub":"user-1","tenant":"local","iss":"gateway","aud":"api","exp":4102444800}`, "secret")

	principal, err := (JWTVerifier{
		Secret:   []byte("secret"),
		Issuer:   "gateway",
		Audience: "api",
		Now:      func() time.Time { return time.Unix(1700000000, 0) },
	}).Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if principal.Subject != "user-1" {
		t.Fatalf("subject = %q, want user-1", principal.Subject)
	}
	if principal.Tenant != "local" {
		t.Fatalf("tenant = %q, want local", principal.Tenant)
	}
}

func TestJWTVerifierRejectsBadSignature(t *testing.T) {
	token := signJWT(t, `{"alg":"HS256","typ":"JWT"}`, `{"sub":"user-1","exp":4102444800}`, "secret")

	_, err := (JWTVerifier{Secret: []byte("other")}).Verify(token)
	if err == nil {
		t.Fatal("Verify returned nil error for bad signature")
	}
}

func signJWT(t *testing.T, header string, payload string, secret string) string {
	t.Helper()

	encodedHeader := base64.RawURLEncoding.EncodeToString([]byte(header))
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signingInput := encodedHeader + "." + encodedPayload

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
