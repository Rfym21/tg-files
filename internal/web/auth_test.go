package web

import (
	"strings"
	"testing"
	"time"
)

func TestSignAndParseSessionToken(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	now := time.Unix(1_700_000_000, 0)
	token, err := signSessionToken(secret, "admin", now, 24*time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if strings.Count(token, ".") != 2 {
		t.Fatalf("token does not have 3 parts: %q", token)
	}

	claims, err := parseSessionToken(secret, token, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Sub != "admin" {
		t.Errorf("sub = %q, want admin", claims.Sub)
	}
	if claims.Exp != now.Add(24*time.Hour).Unix() {
		t.Errorf("exp mismatch")
	}
}

func TestParseSessionTokenExpired(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	now := time.Unix(1_700_000_000, 0)
	token, err := signSessionToken(secret, "admin", now, time.Minute)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := parseSessionToken(secret, token, now.Add(2*time.Minute)); err == nil {
		t.Fatal("expected expired token error")
	}
}

func TestParseSessionTokenBadSignature(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	other := []byte("ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ")
	now := time.Unix(1_700_000_000, 0)
	token, err := signSessionToken(secret, "admin", now, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := parseSessionToken(other, token, now); err == nil {
		t.Fatal("expected signature mismatch error")
	}
}

func TestParseSessionTokenBadFormat(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")
	if _, err := parseSessionToken(secret, "not-a-token", time.Now()); err == nil {
		t.Fatal("expected invalid token error")
	}
}
