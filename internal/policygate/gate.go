// Package policygate models the final, action-scoped authorization decision.
// It is deliberately not wired to PAM, Polkit, sudo, or the privileged probe.
// A caller must supply OS-derived peer identity, a factor verifier, and a
// baseline authorizer that re-checks the existing system policy.
package policygate

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/local/proxilens/internal/policygrant"
)

const MaxRequestTTL = 30 * time.Second

var (
	ErrDenied       = errors.New("policy decision denied")
	ErrInvalid      = errors.New("invalid policy decision request")
	ErrExpired      = errors.New("policy decision request expired")
	ErrReplay       = errors.New("policy decision request already used")
	ErrPeerMismatch = errors.New("kernel peer identity does not match binding")
)

// PeerIdentity must be populated by the receiving privileged process from
// SO_PEERCRED and a trusted logind lookup, never from client-supplied fields.
type PeerIdentity struct {
	UID       uint32
	SessionID string
}

// Request binds a one-shot decision to one exact action and service.
type Request struct {
	Binding  policygrant.Binding
	Nonce    [32]byte
	IssuedAt time.Time
	Expires  time.Time
}

// FactorVerifier validates the opaque factor proof against this exact binding.
// Implementations must verify the Duo transaction server-side and reject stale,
// cancelled, mismatched, or previously consumed transactions.
type FactorVerifier interface {
	VerifyFactor(context.Context, policygrant.Binding, []byte) error
}

// BaselineAuthorizer must independently re-check the pre-existing sudoers or
// Polkit decision. A second factor must never widen that baseline policy.
type BaselineAuthorizer interface {
	Allows(context.Context, policygrant.Binding) (bool, error)
}

type Gate struct {
	factors  FactorVerifier
	baseline BaselineAuthorizer
	now      func() time.Time
	mu       sync.Mutex
	used     map[[32]byte]time.Time
}

func New(f FactorVerifier, b BaselineAuthorizer) *Gate {
	return &Gate{factors: f, baseline: b, now: time.Now, used: make(map[[32]byte]time.Time)}
}

// Check returns nil only when peer identity, freshness, one-time use, factor
// verification, and the existing system policy all pass. It does not execute
// a command or install/modify any system authorization policy.
func (g *Gate) Check(ctx context.Context, r Request, peer PeerIdentity, proof []byte) error {
	if g == nil || g.factors == nil || g.baseline == nil || ctx == nil || len(proof) == 0 || !validBinding(r.Binding) || r.Nonce == ([32]byte{}) {
		return ErrInvalid
	}
	if peer.UID != r.Binding.UID || peer.SessionID == "" || subtle.ConstantTimeCompare([]byte(peer.SessionID), []byte(r.Binding.SessionID)) != 1 {
		return ErrPeerMismatch
	}
	now := g.now()
	if r.IssuedAt.IsZero() || r.Expires.IsZero() || r.Expires.Before(r.IssuedAt) || r.Expires.Sub(r.IssuedAt) > MaxRequestTTL || now.Before(r.IssuedAt.Add(-5*time.Second)) || !now.Before(r.Expires) {
		return ErrExpired
	}
	g.mu.Lock()
	for nonce, expiry := range g.used {
		if !now.Before(expiry) {
			delete(g.used, nonce)
		}
	}
	if _, ok := g.used[r.Nonce]; ok {
		g.mu.Unlock()
		return ErrReplay
	}
	// Consume before external verification: failed proofs cannot be replayed.
	g.used[r.Nonce] = r.Expires
	g.mu.Unlock()

	if err := g.factors.VerifyFactor(ctx, r.Binding, proof); err != nil {
		return ErrDenied
	}
	allowed, err := g.baseline.Allows(ctx, r.Binding)
	if err != nil || !allowed {
		return ErrDenied
	}
	return nil
}

func validBinding(b policygrant.Binding) bool {
	return b.SessionID != "" && b.Service != "" && b.Action != "" &&
		strings.TrimSpace(b.SessionID) == b.SessionID &&
		strings.TrimSpace(b.Service) == b.Service &&
		strings.TrimSpace(b.Action) == b.Action &&
		!strings.ContainsAny(b.SessionID+b.Service+b.Action, "\r\n\x00")
}
