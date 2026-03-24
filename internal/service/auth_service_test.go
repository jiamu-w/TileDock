package service

import "testing"

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "" || hash == "secret123" {
		t.Fatalf("HashPassword() returned invalid hash: %q", hash)
	}
}
