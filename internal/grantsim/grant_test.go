package grantsim

import (
	"testing"
	"time"
)

func TestGrantLifecycle(t *testing.T) {
	var g Grant
	if g.Active() {
		t.Fatal("new grant must be inactive")
	}
	expires := g.Start(15 * time.Minute)
	if !g.Active() {
		t.Fatal("grant should be active")
	}
	if expires.IsZero() || g.ExpiresAt().IsZero() {
		t.Fatal("expiry should be set")
	}
	g.Revoke()
	if g.Active() || !g.ExpiresAt().IsZero() {
		t.Fatal("revoked grant should be inactive")
	}
}

func TestExpiredGrant(t *testing.T) {
	var g Grant
	g.Start(-time.Second)
	if g.Active() {
		t.Fatal("expired grant must be inactive")
	}
}
