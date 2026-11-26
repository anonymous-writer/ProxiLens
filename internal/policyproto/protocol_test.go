package policyproto

import (
	"errors"
	"testing"
	"time"
)

func validRequest(t *testing.T, now time.Time) Request {
	t.Helper()
	r, err := NewRequest(1000, "c3", "org.freedesktop.policykit.exec", now)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestValidateRequestAndRejectReplay(t *testing.T) {
	now := time.Now()
	r := validRequest(t, now)
	peer := PeerIdentity{UID: 1000, SessionID: "c3"}
	s := NewNonceSet()
	if err := s.ValidateRequest(r, peer, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := s.ValidateRequest(r, peer, now.Add(2*time.Second)); !errors.Is(err, ErrReplay) {
		t.Fatalf("expected replay rejection, got %v", err)
	}
}

func TestRejectPeerMismatch(t *testing.T) {
	now := time.Now()
	r := validRequest(t, now)
	if err := MatchPeer(r, PeerIdentity{UID: 0, SessionID: "c3"}); !errors.Is(err, ErrPeerMismatch) {
		t.Fatalf("expected peer mismatch, got %v", err)
	}
}

func TestRejectExpiredAndOverlongRequest(t *testing.T) {
	now := time.Now()
	r := validRequest(t, now)
	if err := ValidateEnvelope(r, r.ExpiresAt); !errors.Is(err, ErrExpired) {
		t.Fatalf("expected expiry, got %v", err)
	}
	r = validRequest(t, now)
	r.Action = string(make([]byte, MaxActionLen+1))
	if err := ValidateEnvelope(r, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected invalid, got %v", err)
	}
}

func TestNonceCanBeUsedAfterExpiryCleanup(t *testing.T) {
	now := time.Now()
	s := NewNonceSet()
	if err := s.Consume("nonce-value", now.Add(time.Second), now); err != nil {
		t.Fatal(err)
	}
	if err := s.Consume("nonce-value", now.Add(time.Minute), now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
}
