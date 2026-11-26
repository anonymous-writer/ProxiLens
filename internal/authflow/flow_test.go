package authflow

import (
	"errors"
	"testing"
)

func TestDuoIsPrimaryAndPasswordBackupIsManual(t *testing.T) {
	f := New()
	if err := f.StartDuo(); err != nil {
		t.Fatal(err)
	}
	if f.State() != DuoPending {
		t.Fatal("Duo should be pending")
	}
	if !errors.Is(f.SelectPassword(), ErrInvalidTransition) {
		t.Fatal("password backup must not be selectable while Duo is pending")
	}
	if err := f.CompleteDuo(true); err != nil {
		t.Fatal(err)
	}
	if f.State() != DuoApproved {
		t.Fatal("Duo approval should be terminal")
	}
	if !errors.Is(f.SelectPassword(), ErrInvalidTransition) {
		t.Fatal("password backup must not replace successful Duo")
	}
}

func TestPasswordBackupRequiresExplicitSelectionAfterDuoFailure(t *testing.T) {
	f := New()
	if err := f.StartDuo(); err != nil {
		t.Fatal(err)
	}
	if err := f.CompleteDuo(false); err != nil {
		t.Fatal(err)
	}
	if f.State() != DuoUnavailable {
		t.Fatal("failed Duo should expose backup choice, not authenticate")
	}
	if err := f.SelectPassword(); err != nil {
		t.Fatal(err)
	}
	if f.State() != PasswordSelected {
		t.Fatal("manual selection not recorded")
	}
	if err := f.CompletePassword(false); err != nil {
		t.Fatal(err)
	}
	if f.State() != Denied {
		t.Fatal("failed PAM result must deny")
	}
}

func TestCancelledDuoAllowsManualBackupOnly(t *testing.T) {
	f := New()
	_ = f.StartDuo()
	if err := f.CancelDuo(); err != nil {
		t.Fatal(err)
	}
	if err := f.SelectPassword(); err != nil {
		t.Fatal(err)
	}
	if f.State() != PasswordSelected {
		t.Fatal("explicit backup selection not recorded")
	}
}

func TestPasswordCannotAuthenticateWithoutSelection(t *testing.T) {
	f := New()
	if !errors.Is(f.CompletePassword(true), ErrInvalidTransition) {
		t.Fatal("password authentication accepted without explicit selection")
	}
}
