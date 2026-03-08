package auth

import (
	"context"
	"testing"
)

func TestVerifyGoogleToken_missingToken(t *testing.T) {
	_, err := VerifyGoogleToken(context.Background(), "", "some-client-id")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestVerifyGoogleToken_missingAudience(t *testing.T) {
	_, err := VerifyGoogleToken(context.Background(), "fake-token", "")
	if err == nil {
		t.Fatal("expected error for empty audience")
	}
}

func TestVerifyGoogleToken_invalidToken(t *testing.T) {
	_, err := VerifyGoogleToken(context.Background(), "invalid.jwt.token", "123456789.apps.googleusercontent.com")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}
