package jwt_test

import (
	"testing"
	"time"

	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/jwt"
)

const testSecret = "test-secret-at-least-16-bytes"

func TestIssueAndVerify_RoundTrip(t *testing.T) {
	issuer, err := jwt.NewIssuer(testSecret)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	verifier, err := jwt.NewVerifier(testSecret)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	token, err := issuer.Issue("user-1", "a@b.com", time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	claims, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.UserID != "user-1" || claims.Email != "a@b.com" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestVerify_RejectsExpiredToken(t *testing.T) {
	issuer, _ := jwt.NewIssuer(testSecret)
	verifier, _ := jwt.NewVerifier(testSecret)

	token, _ := issuer.Issue("user-1", "a@b.com", -time.Hour) // already expired
	if _, err := verifier.Verify(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestVerify_RejectsWrongSecret(t *testing.T) {
	issuer, _ := jwt.NewIssuer(testSecret)
	otherVerifier, _ := jwt.NewVerifier("a-totally-different-secret")

	token, _ := issuer.Issue("user-1", "a@b.com", time.Hour)
	if _, err := otherVerifier.Verify(token); err == nil {
		t.Fatal("expected token signed with a different secret to be rejected")
	}
}

func TestVerify_RejectsGarbage(t *testing.T) {
	verifier, _ := jwt.NewVerifier(testSecret)
	if _, err := verifier.Verify("not-a-jwt"); err == nil {
		t.Fatal("expected garbage token to be rejected")
	}
}

func TestNewIssuer_RejectsShortSecret(t *testing.T) {
	if _, err := jwt.NewIssuer("too-short"); err == nil {
		t.Fatal("expected error for short secret")
	}
}

func TestNewVerifier_RejectsShortSecret(t *testing.T) {
	if _, err := jwt.NewVerifier("too-short"); err == nil {
		t.Fatal("expected error for short secret")
	}
}
