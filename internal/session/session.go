// Package session reads the current user's logind session state.
// It deliberately does not unlock sessions or alter PAM/GDM configuration.
package session

import (
	"fmt"
	"os/exec"
	"os/user"
	"strings"
)

type State struct {
	ID     string
	User   string
	Seat   string
	Type   string
	Class  string
	Active bool
	Locked bool
}

// Current selects exactly one active, graphical Wayland user session on a seat.
// Manager, greeter, background and ambiguous sessions are never selected.
func Current() (State, error) {
	u, err := user.Current()
	if err != nil {
		return State{}, err
	}
	out, err := exec.Command("loginctl", "list-sessions", "--no-legend").Output()
	if err != nil {
		return State{}, fmt.Errorf("loginctl list-sessions: %w", err)
	}

	var matches []State
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		// SESSION UID USER SEAT LEADER CLASS TTY IDLE SINCE
		if len(fields) < 6 || fields[2] != u.Username || fields[3] == "-" || fields[5] != "user" {
			continue
		}

		props, err := exec.Command("loginctl", "show-session", fields[0],
			"-p", "Name", "-p", "Seat", "-p", "Type", "-p", "Class",
			"-p", "Active", "-p", "LockedHint").Output()
		if err != nil {
			continue
		}
		s := State{ID: fields[0], User: u.Username}
		for _, line := range strings.Split(string(props), "\n") {
			p := strings.SplitN(line, "=", 2)
			if len(p) != 2 {
				continue
			}
			switch p[0] {
			case "Name":
				s.User = p[1]
			case "Seat":
				s.Seat = p[1]
			case "Type":
				s.Type = p[1]
			case "Class":
				s.Class = p[1]
			case "Active":
				s.Active = p[1] == "yes"
			case "LockedHint":
				s.Locked = p[1] == "yes"
			}
		}
		if s.User == u.Username && s.Seat != "" && s.Class == "user" &&
			s.Type == "wayland" && s.Active {
			matches = append(matches, s)
		}
	}

	if len(matches) == 0 {
		return State{}, fmt.Errorf("keine aktive Wayland-Benutzersitzung auf einem Seat für %s gefunden", u.Username)
	}
	if len(matches) != 1 {
		return State{}, fmt.Errorf("mehrere aktive Wayland-Sitzungen für %s gefunden; aus Sicherheitsgründen keine Auswahl", u.Username)
	}
	return matches[0], nil
}

// UnlockCurrent unlocks the current user's active Wayland session only when
// it is already locked. It refuses other session classes and desktop types.
func UnlockCurrent() error {
	s, err := Current()
	if err != nil {
		return err
	}
	if s.User == "" || s.ID == "" || s.Seat == "" {
		return fmt.Errorf("Sitzungsidentität ist unvollständig")
	}
	if s.Class != "user" {
		return fmt.Errorf("Sitzungsklasse %q ist nicht erlaubt", s.Class)
	}
	if s.Type != "wayland" {
		return fmt.Errorf("Sitzungstyp %q ist nicht Wayland", s.Type)
	}
	if !s.Active {
		return fmt.Errorf("Sitzung %s ist nicht aktiv", s.ID)
	}
	if !s.Locked {
		return fmt.Errorf("Sitzung %s ist nicht gesperrt; Entsperrtest abgebrochen", s.ID)
	}
	if out, err := exec.Command("loginctl", "unlock-session", s.ID).CombinedOutput(); err != nil {
		return fmt.Errorf("loginctl unlock-session %s: %w: %s", s.ID, err, strings.TrimSpace(string(out)))
	}
	return nil
}
