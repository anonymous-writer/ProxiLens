package approval

import (
	"context"
	"errors"
	"fmt"

	"github.com/local/proxilens/internal/duo"
	"github.com/local/proxilens/internal/session"
)

// DuoPushFunc is injectable for tests. Production uses DuoConfig.Push.
type DuoPushFunc func(context.Context, func(string)) error

// DuoAdapter connects the coordinator to the existing Duo Push client.
// It authenticates only; it does not create a grant or authorize an action.
type DuoAdapter struct {
	Config   duo.Config
	Progress func(string)
	PushFunc DuoPushFunc
}

func (a DuoAdapter) Push(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if a.PushFunc != nil {
		return a.PushFunc(ctx, a.Progress)
	}
	return a.Config.Push(ctx, a.Progress)
}

// CurrentSessionFunc allows session inspection to be tested without invoking
// loginctl. The production default reads the caller's active logind session.
type CurrentSessionFunc func() (session.State, error)

// LogindSessionInspector checks that exactly one active graphical user
// session is observable. This is diagnostic context, NOT a security identity
// assertion for a privileged daemon and NOT an authorization decision.
type LogindSessionInspector struct {
	Current CurrentSessionFunc
}

func (i LogindSessionInspector) InspectCurrentSession(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	current := i.Current
	if current == nil {
		current = session.Current
	}
	s, err := current()
	if err != nil {
		return fmt.Errorf("aktive Benutzersitzung konnte nicht geprüft werden: %w", err)
	}
	if s.ID == "" || s.User == "" || s.Seat == "" {
		return errors.New("Sitzungsdaten unvollständig; keine Freigabe")
	}
	if s.Class != "user" || s.Type != "wayland" || !s.Active {
		return errors.New("keine aktive Wayland-Benutzersitzung; keine Freigabe")
	}
	return nil
}
