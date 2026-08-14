package auth_test

import (
	"strings"
	"testing"

	"kmainstay/internal/auth"
)

func TestPassword_Argon2idRoundTripAndWrongPassword(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash = %q", hash)
	}
	if !auth.VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("correct password rejected")
	}
	if auth.VerifyPassword(hash, "wrong") {
		t.Fatal("wrong password accepted")
	}
}

func TestSecret_SHA256RoundTripAndWrongSecret(t *testing.T) {
	hash := auth.HashSecret("random-api-key-secret")
	if !strings.HasPrefix(hash, "$sha256$") {
		t.Fatalf("hash = %q", hash)
	}
	if !auth.VerifySecret(hash, "random-api-key-secret") {
		t.Fatal("correct secret rejected")
	}
	if auth.VerifySecret(hash, "wrong-secret") {
		t.Fatal("wrong secret accepted")
	}
	if auth.VerifySecret("not-a-secret-hash", "random-api-key-secret") {
		t.Fatal("malformed hash accepted")
	}
}
