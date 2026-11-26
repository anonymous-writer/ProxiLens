// Package policygrant models short-lived, action-scoped approvals.
// It is a policy-core prototype only: callers must authenticate factors and
// derive UID/session/service from trusted operating-system interfaces before
// issuing a grant. This package does not integrate with PAM, Polkit or sudo.
package policygrant

import (
	"errors"
	"strings"
	"sync"
	"time"
)

const DefaultTTL = 15 * time.Minute

var (
	ErrInvalidBinding = errors.New("incomplete or invalid approval binding")
	ErrNotApproved    = errors.New("no matching active approval")
)

type Binding struct {
	UID       uint32
	SessionID string
	Service   string // e.g. "sudo" or "polkit"
	Action    string // exact sudo command or Polkit action ID
}

type Approval struct {
	Binding  Binding
	IssuedAt time.Time
	Expires  time.Time
}

type Store struct {
	mu    sync.Mutex
	items map[Binding]Approval
	now   func() time.Time
}

func New() *Store { return &Store{items: make(map[Binding]Approval), now: time.Now} }

// Issue creates an in-memory, action-specific approval. It MUST only be called
// after a trusted authentication flow has completed successfully. The caller
// is responsible for deriving Binding from kernel/logind/PAM data, not UI input.
func (s *Store) Issue(b Binding, ttl time.Duration) (Approval, error) {
	if s == nil || !valid(b) {
		return Approval{}, ErrInvalidBinding
	}
	if ttl <= 0 || ttl > DefaultTTL {
		ttl = DefaultTTL
	}
	now := s.now()
	a := Approval{Binding: b, IssuedAt: now, Expires: now.Add(ttl)}
	s.mu.Lock()
	s.items[b] = a
	s.mu.Unlock()
	return a, nil
}

func (s *Store) Check(b Binding) error {
	if s == nil || !valid(b) {
		return ErrInvalidBinding
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.items[b]
	if !ok || !s.now().Before(a.Expires) {
		delete(s.items, b)
		return ErrNotApproved
	}
	return nil
}

// Consume atomically checks and removes an approval. Exactly one concurrent
// caller can consume a given binding. A consumed approval cannot be replayed.
func (s *Store) Consume(b Binding) (Approval, error) {
	if s == nil || !valid(b) {
		return Approval{}, ErrInvalidBinding
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.items[b]
	if !ok {
		return Approval{}, ErrNotApproved
	}
	delete(s.items, b)
	if !s.now().Before(a.Expires) {
		return Approval{}, ErrNotApproved
	}
	return a, nil
}

func (s *Store) Revoke(b Binding) {
	if s == nil {
		return
	}
	s.mu.Lock()
	delete(s.items, b)
	s.mu.Unlock()
}

func (s *Store) RevokeSession(uid uint32, sessionID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	for b := range s.items {
		if b.UID == uid && b.SessionID == sessionID {
			delete(s.items, b)
		}
	}
	s.mu.Unlock()
}

func (s *Store) RevokeAll() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.items = make(map[Binding]Approval)
	s.mu.Unlock()
}

func valid(b Binding) bool {
	return b.SessionID != "" && b.Service != "" && b.Action != "" &&
		strings.TrimSpace(b.SessionID) == b.SessionID &&
		strings.TrimSpace(b.Service) == b.Service &&
		strings.TrimSpace(b.Action) == b.Action &&
		!strings.ContainsAny(b.SessionID+b.Service+b.Action, "\r\n\x00")
}
