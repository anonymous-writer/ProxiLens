package approval

import (
	"context"
	"errors"
	"testing"
)

type duoStub struct{ err error }

func (s duoStub) Push(context.Context) error { return s.err }

type passwordStub struct {
	calls int
	err   error
}

func (s *passwordStub) AuthenticateCurrentUser(context.Context) error { s.calls++; return s.err }

type sessionStub struct {
	calls int
	err   error
}

func (s *sessionStub) InspectCurrentSession(context.Context) error { s.calls++; return s.err }
func TestDuoSuccessNeverAuthorizes(t *testing.T) {
	s := &sessionStub{}
	c := New(duoStub{}, nil, s)
	r, err := c.DuoPush(context.Background())
	if !errors.Is(err, ErrNotAuthorizing) || !r.Authenticated || r.Authorized || !r.SessionInspected || s.calls != 1 {
		t.Fatalf("result=%+v err=%v", r, err)
	}
}
func TestDuoFailureDoesNotSelectPassword(t *testing.T) {
	p := &passwordStub{}
	c := New(duoStub{err: errors.New("denied")}, p, nil)
	if _, err := c.DuoPush(context.Background()); err == nil {
		t.Fatal("expected Duo error")
	}
	if p.calls != 0 {
		t.Fatal("password fallback ran automatically")
	}
}
func TestPasswordFallbackRequiresExplicitCallAfterDuoFailure(t *testing.T) {
	p := &passwordStub{}
	c := New(duoStub{err: errors.New("offline")}, p, nil)

	// First perform the primary Duo attempt. A configured Duo error moves the
	// state machine to DuoUnavailable, but must not invoke the password adapter.
	if _, err := c.DuoPush(context.Background()); err == nil {
		t.Fatal("expected Duo failure")
	}
	if p.calls != 0 {
		t.Fatal("password fallback ran automatically after Duo failure")
	}

	// The separate call models the user's explicit selection of password backup.
	r, err := c.ManualPasswordFallback(context.Background())
	if !errors.Is(err, ErrNotAuthorizing) {
		t.Fatalf("expected authenticated-but-non-authorizing result, got result=%+v err=%v", r, err)
	}
	if r.Method != MethodPassword || !r.Authenticated || r.Authorized {
		t.Fatalf("unexpected manual fallback result: %+v", r)
	}
	if p.calls != 1 {
		t.Fatalf("password calls=%d", p.calls)
	}
}
func TestPasswordFallbackUnavailableBeforeDuoFailure(t *testing.T) {
	p := &passwordStub{}
	c := New(duoStub{}, p, nil)
	if _, err := c.ManualPasswordFallback(context.Background()); err == nil {
		t.Fatal("fallback must not be selectable before Duo failure")
	}
	if p.calls != 0 {
		t.Fatal("password authenticator unexpectedly called")
	}
}
