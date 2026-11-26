package approval

import (
	"context"
	"errors"
	"testing"

	"github.com/local/proxilens/internal/session"
)

func TestDuoAdapterPassesProgressAndContext(t *testing.T) {
	want := errors.New("Duo transport failed")
	called := false
	progress := func(string) {}
	a := DuoAdapter{Progress: progress, PushFunc: func(ctx context.Context, got func(string)) error {
		called = true
		if got == nil {
			t.Fatal("progress callback was not passed")
		}
		return want
	}}
	if err := a.Push(context.Background()); !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
	if !called {
		t.Fatal("Duo adapter did not invoke push")
	}
}

func TestDuoAdapterHonorsCancelledContextBeforeNetwork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	a := DuoAdapter{PushFunc: func(context.Context, func(string)) error { called = true; return nil }}
	if err := a.Push(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if called {
		t.Fatal("push called after cancellation")
	}
}

func TestLogindInspectorRequiresActiveGraphicalUserSession(t *testing.T) {
	valid := session.State{ID: "4", User: "tester", Seat: "seat0", Class: "user", Type: "wayland", Active: true}
	inspector := LogindSessionInspector{Current: func() (session.State, error) { return valid, nil }}
	if err := inspector.InspectCurrentSession(context.Background()); err != nil {
		t.Fatalf("valid session rejected: %v", err)
	}

	valid.Active = false
	if err := inspector.InspectCurrentSession(context.Background()); err == nil {
		t.Fatal("inactive session accepted")
	}
}

func TestLogindInspectorRejectsAmbiguousOrUnavailableSession(t *testing.T) {
	inspector := LogindSessionInspector{Current: func() (session.State, error) { return session.State{}, errors.New("ambiguous") }}
	if err := inspector.InspectCurrentSession(context.Background()); err == nil {
		t.Fatal("unavailable session accepted")
	}
}
