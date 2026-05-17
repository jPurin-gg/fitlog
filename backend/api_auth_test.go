package main

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := hashPassword("secret-password")
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}

	ok, err := verifyPassword("secret-password", hash)
	if err != nil {
		t.Fatalf("verifyPassword() error = %v", err)
	}
	if !ok {
		t.Fatal("verifyPassword() = false, want true")
	}

	ok, err = verifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("verifyPassword() wrong password error = %v", err)
	}
	if ok {
		t.Fatal("verifyPassword() wrong password = true, want false")
	}
}
