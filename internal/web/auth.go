package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const jwtHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"

type sessionClaims struct {
	Sub string `json:"sub"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
	Jti string `json:"jti"`
}

var (
	errInvalidToken = errors.New("invalid token")
	errExpiredToken = errors.New("token expired")
)

func signSessionToken(secret []byte, user string, now time.Time, ttl time.Duration) (string, error) {
	jtiBytes := make([]byte, 8)
	if _, err := rand.Read(jtiBytes); err != nil {
		return "", err
	}
	claims := sessionClaims{
		Sub: user,
		Iat: now.Unix(),
		Exp: now.Add(ttl).Unix(),
		Jti: hex.EncodeToString(jtiBytes),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	unsigned := jwtHeader + "." + encoded
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(unsigned))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + sig, nil
}

func parseSessionToken(secret []byte, token string, now time.Time) (sessionClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return sessionClaims{}, errInvalidToken
	}
	if parts[0] != jwtHeader {
		return sessionClaims{}, errInvalidToken
	}
	unsigned := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(unsigned))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return sessionClaims{}, errInvalidToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return sessionClaims{}, fmt.Errorf("%w: payload base64", errInvalidToken)
	}
	var claims sessionClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return sessionClaims{}, fmt.Errorf("%w: payload json", errInvalidToken)
	}
	if claims.Exp < now.Unix() {
		return sessionClaims{}, errExpiredToken
	}
	return claims, nil
}
