package policygrant

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestApprovalIsBoundToEveryField(t *testing.T) {
	s := New()
	b := Binding{UID: 1000, SessionID: "1", Service: "sudo", Action: "/usr/bin/apt update"}
	if _, err := s.Issue(b, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := s.Check(b); err != nil {
		t.Fatal(err)
	}
	variants := []Binding{
		{UID: 1001, SessionID: "1", Service: "sudo", Action: b.Action},
		{UID: 1000, SessionID: "2", Service: "sudo", Action: b.Action},
		{UID: 1000, SessionID: "1", Service: "polkit", Action: b.Action},
		{UID: 1000, SessionID: "1", Service: "sudo", Action: "/usr/bin/id"},
	}
	for _, v := range variants {
		if err := s.Check(v); !errors.Is(err, ErrNotApproved) {
			t.Fatalf("binding %#v unexpectedly matched: %v", v, err)
		}
	}
}

func TestExpiryAndRevocation(t *testing.T) {
	s := New()
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	b := Binding{UID: 1000, SessionID: "1", Service: "polkit", Action: "org.example.test"}
	if _, err := s.Issue(b, time.Minute); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if !errors.Is(s.Check(b), ErrNotApproved) {
		t.Fatal("expired approval must be rejected")
	}
	if _, err := s.Issue(b, time.Minute); err != nil {
		t.Fatal(err)
	}
	s.RevokeSession(1000, "1")
	if !errors.Is(s.Check(b), ErrNotApproved) {
		t.Fatal("session revoke must clear approval")
	}
}

func TestInvalidBindingsRejected(t *testing.T) {
	s := New()
	for _, b := range []Binding{{}, {UID: 1000, SessionID: "1", Service: "sudo", Action: "x\n"}} {
		if _, err := s.Issue(b, time.Minute); !errors.Is(err, ErrInvalidBinding) {
			t.Fatalf("expected invalid binding, got %v", err)
		}
	}
}

func TestConsumeIsSingleUse(t *testing.T) {
	s := New()
	b := Binding{UID: 1000, SessionID: "c3", Service: "polkit", Action: "org.example.action"}
	if _, err := s.Issue(b, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Consume(b); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if _, err := s.Consume(b); !errors.Is(err, ErrNotApproved) {
		t.Fatalf("replay must fail, got %v", err)
	}
}

func TestConcurrentConsumeHasOneWinner(t *testing.T) {
	s := New()
	b := Binding{UID: 1000, SessionID: "c3", Service: "sudo", Action: "/usr/bin/id"}
	if _, err := s.Issue(b, time.Minute); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Consume(b); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("expected exactly one consumer, got %d", wins)
	}
}
