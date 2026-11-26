// Package authflow models ProxiLens' primary-Duo, explicitly selected password
// backup flow. It contains no PAM implementation and never handles passwords.
package authflow

import (
	"errors"
	"sync"
)

type State uint8

const (
	Idle State = iota
	DuoPending
	DuoApproved
	DuoUnavailable
	PasswordSelected
	PasswordAuthenticated
	Denied
	Cancelled
)

var ErrInvalidTransition = errors.New("invalid authentication flow transition")

type Flow struct {
	mu    sync.Mutex
	state State
}

func New() *Flow { return &Flow{state: Idle} }

func (f *Flow) State() State {
	if f == nil {
		return Denied
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

func (f *Flow) StartDuo() error {
	if f == nil {
		return ErrInvalidTransition
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state == DuoPending {
		return ErrInvalidTransition
	}
	f.state = DuoPending
	return nil
}

// CompleteDuo records the terminal Duo result. A failed/unavailable request
// exposes the password backup as a separate user action; it never selects it.
func (f *Flow) CompleteDuo(approved bool) error {
	if f == nil {
		return ErrInvalidTransition
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state != DuoPending {
		return ErrInvalidTransition
	}
	if approved {
		f.state = DuoApproved
	} else {
		f.state = DuoUnavailable
	}
	return nil
}

func (f *Flow) CancelDuo() error {
	if f == nil {
		return ErrInvalidTransition
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state != DuoPending {
		return ErrInvalidTransition
	}
	f.state = Cancelled
	return nil
}

// SelectPassword requires an explicit user action after Duo did not complete.
func (f *Flow) SelectPassword() error {
	if f == nil {
		return ErrInvalidTransition
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state != DuoUnavailable && f.state != Cancelled {
		return ErrInvalidTransition
	}
	f.state = PasswordSelected
	return nil
}

// CompletePassword is called only by a future trusted PAM adapter. This
// package itself does not authenticate or collect a password.
func (f *Flow) CompletePassword(authenticated bool) error {
	if f == nil {
		return ErrInvalidTransition
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state != PasswordSelected {
		return ErrInvalidTransition
	}
	if authenticated {
		f.state = PasswordAuthenticated
	} else {
		f.state = Denied
	}
	return nil
}

func (f *Flow) Reset() {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.state = Idle
	f.mu.Unlock()
}
