package larkauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const oauthStateTTLSeconds = 600

// GenerateOAuthState builds a signed nonce.timestamp.sig token (same idea as aiops FastAPI).
func GenerateOAuthState(secret string) string {
	if secret == "" {
		secret = "sya_oauth_state_fallback"
	}
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	nonce := base64.RawURLEncoding.EncodeToString(b)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	payload := nonce + "." + ts
	sig := hmacSHA256(secret, payload)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return payload + "." + sigB64
}

func hmacSHA256(secret, payload string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

// ValidateOAuthState checks query state matches cookie value and signature + TTL.
func ValidateOAuthState(secret, cookieState, queryState string) error {
	cs := strings.TrimSpace(cookieState)
	qs := strings.TrimSpace(queryState)
	if len(cs) != len(qs) || subtle.ConstantTimeCompare([]byte(cs), []byte(qs)) != 1 {
		return fmt.Errorf("invalid OAuth state")
	}
	parts := strings.Split(qs, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid OAuth state format")
	}
	nonce, tsText, providedSig := parts[0], parts[1], parts[2]
	if nonce == "" || tsText == "" || providedSig == "" {
		return fmt.Errorf("invalid OAuth state")
	}
	ts, err := strconv.ParseInt(tsText, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid OAuth state")
	}
	now := time.Now().Unix()
	if now-ts > oauthStateTTLSeconds {
		return fmt.Errorf("OAuth state expired")
	}
	if ts-now > 60 {
		return fmt.Errorf("invalid OAuth state")
	}
	if secret == "" {
		secret = "sya_oauth_state_fallback"
	}
	payload := nonce + "." + tsText
	expected := base64.RawURLEncoding.EncodeToString(hmacSHA256(secret, payload))
	if len(expected) != len(providedSig) || subtle.ConstantTimeCompare([]byte(expected), []byte(providedSig)) != 1 {
		return fmt.Errorf("invalid OAuth state signature")
	}
	return nil
}
