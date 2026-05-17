package auth

import "testing"

func TestHashPasswordAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	if !CheckPassword("correct-password", hash) {
		t.Fatal("expected password to match hash")
	}

	if CheckPassword("wrong-password", hash) {
		t.Fatal("expected wrong password not to match hash")
	}
}

func TestNewSessionTokenAndHashToken(t *testing.T) {
	token, err := NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken returned error: %v", err)
	}
	if token == "" {
		t.Fatal("expected token")
	}
	if HashToken(token) == token {
		t.Fatal("expected token hash to differ from token")
	}
}
