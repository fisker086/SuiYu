package larkauth

import "testing"

func TestOAuthStateRoundTrip(t *testing.T) {
	secret := "test-secret-key"
	s := GenerateOAuthState(secret)
	if err := ValidateOAuthState(secret, s, s); err != nil {
		t.Fatal(err)
	}
	if err := ValidateOAuthState(secret, s, s+"x"); err == nil {
		t.Fatal("expected mismatch")
	}
}
