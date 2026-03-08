package auth

import (
	"context"
	"fmt"

	"google.golang.org/api/idtoken"
)

// Verifier validates Google ID tokens. Use for testing with mocks.
type Verifier interface {
	Verify(ctx context.Context, token, audience string) (*idtoken.Payload, error)
}

// GoogleVerifier uses the real Google idtoken package.
type GoogleVerifier struct{}

func (GoogleVerifier) Verify(ctx context.Context, token, audience string) (*idtoken.Payload, error) {
	return VerifyGoogleToken(ctx, token, audience)
}

// VerifyGoogleToken validates a Google ID token and returns the payload.
// audience is the OAuth2 client ID (e.g. from GOOGLE_CLIENT_ID env).
func VerifyGoogleToken(ctx context.Context, token string, audience string) (*idtoken.Payload, error) {
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}
	if audience == "" {
		return nil, fmt.Errorf("missing audience (GOOGLE_CLIENT_ID)")
	}
	payload, err := idtoken.Validate(ctx, token, audience)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	return payload, nil
}
