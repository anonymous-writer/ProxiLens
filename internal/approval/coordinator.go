// Package approval composes the authentication flow with injectable Duo, PAM,
// and policy-session diagnostics. It is deliberately non-authorizing: a
// successful result is an authentication/diagnostic outcome, never a system grant.
package approval

import (
	"context"
	"errors"

	"github.com/local/proxilens/internal/authflow"
)

var ErrNotAuthorizing = errors.New("Beta 10.5 is a non-authorizing integration preview")

// DuoAuthenticator performs a Duo Push transaction.
type DuoAuthenticator interface{ Push(context.Context) error }

// PasswordAuthenticator is an adapter boundary for a future PAM integration.
// Implementations must prompt interactively and must never accept a password
// supplied by the GUI, environment, command line, or an IPC request.
type PasswordAuthenticator interface{ AuthenticateCurrentUser(context.Context) error }

// SessionInspector may only inspect the caller's session; it cannot grant access.
type SessionInspector interface{ InspectCurrentSession(context.Context) error }

type Method string

const (
	MethodDuo      Method = "duo-push"
	MethodPassword Method = "manual-password"
)

type Result struct {
	Method           Method
	Authenticated    bool
	SessionInspected bool
	Authorized       bool // Always false in Beta 10.5.
}

type Coordinator struct {
	flow     *authflow.Flow
	duo      DuoAuthenticator
	password PasswordAuthenticator
	session  SessionInspector
}

func New(d DuoAuthenticator, p PasswordAuthenticator, s SessionInspector) *Coordinator {
	return &Coordinator{flow: authflow.New(), duo: d, password: p, session: s}
}

func (c *Coordinator) DuoPush(ctx context.Context) (Result, error) {
	if c == nil || c.flow == nil || c.duo == nil {
		return Result{}, errors.New("Duo authenticator unavailable")
	}
	if err := c.flow.StartDuo(); err != nil {
		return Result{}, err
	}
	err := c.duo.Push(ctx)
	if err != nil {
		_ = c.flow.CompleteDuo(false)
		return Result{Method: MethodDuo}, err
	}
	if err := c.flow.CompleteDuo(true); err != nil {
		return Result{}, err
	}
	return c.finish(ctx, MethodDuo)
}

// ManualPasswordFallback is intentionally a separate entry point. Callers
// must invoke it only in response to an explicit user selection in the UI.
func (c *Coordinator) ManualPasswordFallback(ctx context.Context) (Result, error) {
	if c == nil || c.flow == nil || c.password == nil {
		return Result{}, errors.New("manual password authentication unavailable")
	}
	if err := c.flow.SelectPassword(); err != nil {
		return Result{}, err
	}
	if err := c.password.AuthenticateCurrentUser(ctx); err != nil {
		_ = c.flow.CompletePassword(false)
		return Result{Method: MethodPassword}, err
	}
	if err := c.flow.CompletePassword(true); err != nil {
		return Result{}, err
	}
	return c.finish(ctx, MethodPassword)
}

func (c *Coordinator) finish(ctx context.Context, method Method) (Result, error) {
	r := Result{Method: method, Authenticated: true, Authorized: false}
	if c.session != nil {
		if err := c.session.InspectCurrentSession(ctx); err != nil {
			return r, err
		}
		r.SessionInspected = true
	}
	return r, ErrNotAuthorizing
}
