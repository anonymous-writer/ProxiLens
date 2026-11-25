// Package grantsim provides an in-memory-only grant for UI and workflow tests.
// It is deliberately not connected to PAM, Polkit, sudo, D-Bus or system services.
package grantsim

import (
	"sync"
	"time"
)

type Grant struct {
	mu      sync.Mutex
	expires time.Time
}

// Start begins a simulated grant for the requested duration.
func (g *Grant) Start(duration time.Duration) time.Time {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.expires = time.Now().Add(duration)
	return g.expires
}

// Active reports whether the simulated grant is still active.
func (g *Grant) Active() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return time.Now().Before(g.expires)
}

// ExpiresAt returns the expiry time, or zero when no grant is active.
func (g *Grant) ExpiresAt() time.Time {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !time.Now().Before(g.expires) {
		return time.Time{}
	}
	return g.expires
}

// Revoke clears the simulated grant.
func (g *Grant) Revoke() {
	g.mu.Lock()
	g.expires = time.Time{}
	g.mu.Unlock()
}
