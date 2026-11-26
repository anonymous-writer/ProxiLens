// Package policyproto defines a strict, unprivileged protocol model for the
// future ProxiLens policy service. It does not grant privileges, verify Duo,
// inspect logind sessions, or open an IPC endpoint.
package policyproto

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	Version       = 1
	MaxMessage    = 4096
	MaxActionLen  = 256
	MaxSessionLen = 128
	MaxNonceLen   = 64
	MaxRequestTTL = 30 * time.Second
)

var (
	ErrInvalid       = errors.New("invalid policy protocol message")
	ErrExpired       = errors.New("policy protocol request expired")
	ErrReplay        = errors.New("policy protocol nonce already used")
	ErrPeerMismatch  = errors.New("operating-system peer identity mismatch")
	ErrNotAuthorized = errors.New("request is not authorized")
)

// Request is a request envelope. UID and SessionID are assertions only;
// a future privileged server MUST compare them with OS-derived peer/session data.
type Request struct {
	Version   int       `json:"version"`
	ID        string    `json:"id"`
	Nonce     string    `json:"nonce"`
	UID       uint32    `json:"uid"`
	SessionID string    `json:"session_id"`
	Action    string    `json:"action"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// PeerIdentity must be populated by the server from operating-system APIs,
// never copied from Request fields.
type PeerIdentity struct {
	UID       uint32
	SessionID string
}

// NewRequest creates a short-lived envelope. It is not an authorization token.
func NewRequest(uid uint32, sessionID, action string, now time.Time) (Request, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return Request{}, fmt.Errorf("create nonce: %w", err)
	}
	nonce := hex.EncodeToString(raw[:])
	return Request{
		Version: Version, ID: nonce[:16], Nonce: nonce, UID: uid,
		SessionID: sessionID, Action: action,
		IssuedAt: now.UTC(), ExpiresAt: now.Add(MaxRequestTTL).UTC(),
	}, nil
}

// ValidateEnvelope checks syntax and time bounds. The caller must separately
// verify PeerIdentity and the actual authorization policy.
func ValidateEnvelope(r Request, now time.Time) error {
	if r.Version != Version || r.ID == "" || len(r.ID) > 64 ||
		len(r.Nonce) < 32 || len(r.Nonce) > MaxNonceLen ||
		len(r.SessionID) == 0 || len(r.SessionID) > MaxSessionLen ||
		len(r.Action) == 0 || len(r.Action) > MaxActionLen ||
		strings.TrimSpace(r.Action) != r.Action || strings.ContainsAny(r.Action, "\r\n\x00") ||
		r.IssuedAt.IsZero() || r.ExpiresAt.IsZero() {
		return ErrInvalid
	}
	if r.ExpiresAt.Before(r.IssuedAt) || r.ExpiresAt.Sub(r.IssuedAt) > MaxRequestTTL ||
		now.Before(r.IssuedAt.Add(-5*time.Second)) || !now.Before(r.ExpiresAt) {
		return ErrExpired
	}
	return nil
}

// MatchPeer compares claims with identity obtained independently by the server.
func MatchPeer(r Request, peer PeerIdentity) error {
	if r.UID != peer.UID || r.SessionID != peer.SessionID || peer.SessionID == "" {
		return ErrPeerMismatch
	}
	return nil
}

// NonceSet provides process-local replay protection. A privileged service must
// own its own instance and must not share it with the desktop GUI.
type NonceSet struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

func NewNonceSet() *NonceSet { return &NonceSet{seen: make(map[string]time.Time)} }

// Consume atomically marks a nonce as used until its expiry.
func (s *NonceSet) Consume(nonce string, expiresAt, now time.Time) error {
	if s == nil || nonce == "" || len(nonce) > MaxNonceLen || !now.Before(expiresAt) {
		return ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, expiry := range s.seen {
		if !now.Before(expiry) {
			delete(s.seen, key)
		}
	}
	if _, exists := s.seen[nonce]; exists {
		return ErrReplay
	}
	s.seen[nonce] = expiresAt
	return nil
}

// ValidateRequest performs envelope, OS-peer, and replay checks. A successful
// result means only that the envelope is well-formed and fresh; it does NOT
// mean the requested action is permitted.
func (s *NonceSet) ValidateRequest(r Request, peer PeerIdentity, now time.Time) error {
	if err := ValidateEnvelope(r, now); err != nil {
		return err
	}
	if err := MatchPeer(r, peer); err != nil {
		return err
	}
	return s.Consume(r.Nonce, r.ExpiresAt, now)
}
