package policygate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/local/proxilens/internal/policygrant"
)

type factorStub struct {
	err   error
	calls int
}

func (f *factorStub) VerifyFactor(_ context.Context, _ policygrant.Binding, proof []byte) error {
	f.calls++
	if string(proof) != "verified-proof" {
		return errors.New("bad proof")
	}
	return f.err
}

type baselineStub struct {
	allowed bool
	err     error
	calls   int
}

func (b *baselineStub) Allows(_ context.Context, _ policygrant.Binding) (bool, error) {
	b.calls++
	return b.allowed, b.err
}

func fixture() (*Gate, Request, PeerIdentity, *factorStub, *baselineStub) {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	f, b := &factorStub{}, &baselineStub{allowed: true}
	g := New(f, b)
	g.now = func() time.Time { return now }
	r := Request{Binding: policygrant.Binding{UID: 1000, SessionID: "3", Service: "sudo", Action: "/usr/bin/apt update"}, IssuedAt: now, Expires: now.Add(20 * time.Second)}
	r.Nonce[0] = 1
	return g, r, PeerIdentity{UID: 1000, SessionID: "3"}, f, b
}

func TestCheckRequiresAllIndependentConditions(t *testing.T) {
	g, r, p, f, b := fixture()
	if err := g.Check(context.Background(), r, p, []byte("verified-proof")); err != nil {
		t.Fatal(err)
	}
	if f.calls != 1 || b.calls != 1 {
		t.Fatalf("factor/baseline calls = %d/%d", f.calls, b.calls)
	}
}

func TestRejectsPeerMismatchAndReplay(t *testing.T) {
	g, r, p, _, _ := fixture()
	bad := p
	bad.UID++
	if !errors.Is(g.Check(context.Background(), r, bad, []byte("verified-proof")), ErrPeerMismatch) {
		t.Fatal("peer mismatch accepted")
	}
	if err := g.Check(context.Background(), r, p, []byte("verified-proof")); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(g.Check(context.Background(), r, p, []byte("verified-proof")), ErrReplay) {
		t.Fatal("replay accepted")
	}
}

func TestFailsClosedOnFactorOrBaselineFailure(t *testing.T) {
	g, r, p, f, b := fixture()
	f.err = errors.New("factor failed")
	if !errors.Is(g.Check(context.Background(), r, p, []byte("verified-proof")), ErrDenied) {
		t.Fatal("factor failure did not deny")
	}
	g, r, p, f, b = fixture()
	b.allowed = false
	if !errors.Is(g.Check(context.Background(), r, p, []byte("verified-proof")), ErrDenied) {
		t.Fatal("baseline denial did not deny")
	}
	g, r, p, f, b = fixture()
	b.err = errors.New("policy backend unavailable")
	if !errors.Is(g.Check(context.Background(), r, p, []byte("verified-proof")), ErrDenied) {
		t.Fatal("baseline error did not deny")
	}
}

func TestRejectsExpiredAndMalformedRequests(t *testing.T) {
	g, r, p, _, _ := fixture()
	r.Expires = r.IssuedAt
	if !errors.Is(g.Check(context.Background(), r, p, []byte("verified-proof")), ErrExpired) {
		t.Fatal("expired request accepted")
	}
	g, r, p, _, _ = fixture()
	r.Binding.Action = " /usr/bin/id"
	if !errors.Is(g.Check(context.Background(), r, p, []byte("verified-proof")), ErrInvalid) {
		t.Fatal("malformed binding accepted")
	}
}

func TestFailedFactorConsumesNonce(t *testing.T) {
	g, r, p, f, _ := fixture()
	f.err = errors.New("factor verification failed")
	if !errors.Is(g.Check(context.Background(), r, p, []byte("verified-proof")), ErrDenied) {
		t.Fatal("failed factor did not deny")
	}
	f.err = nil
	if !errors.Is(g.Check(context.Background(), r, p, []byte("verified-proof")), ErrReplay) {
		t.Fatal("failed factor request nonce was reusable")
	}
}

func TestCancelledContextFailsClosed(t *testing.T) {
	g, r, p, f, _ := fixture()
	f.err = context.Canceled
	if !errors.Is(g.Check(context.Background(), r, p, []byte("verified-proof")), ErrDenied) {
		t.Fatal("cancelled verification did not deny")
	}
}
