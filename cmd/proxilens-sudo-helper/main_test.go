//go:build linux

package main

import (
	"strings"
	"testing"
)

func TestDuoOnlyPAMReplacesPasswordAuthentication(t *testing.T) {
	input := "auth sufficient pam_rootok.so\n@include common-auth\n@include common-account\n"
	got := duoOnlyPAM(input, "michael")
	if strings.Contains(got, "@include common-auth") || !strings.Contains(got, "auth substack common-auth") {
		t.Fatalf("conditional Ubuntu password substack missing:\n%s", got)
	}
	if !strings.Contains(got, "auth [success=1 default=ignore] pam_succeed_if.so user = michael") {
		t.Fatalf("configured-user password bypass missing:\n%s", got)
	}
	if !strings.Contains(got, pamLine) || !strings.Contains(got, "@include common-account") {
		t.Fatalf("Duo rule or account stack missing:\n%s", got)
	}
	if again := duoOnlyPAM(got, "michael"); again != got {
		t.Fatal("Duo-only PAM rule was duplicated")
	}
}

func TestPAMRuleUpgradeReplacesLegacySeteuidRule(t *testing.T) {
	input := "@include common-auth\n" + pamMarker + "\nauth required pam_exec.so quiet seteuid /usr/lib/proxilens/proxilens-sudo-helper pam-auth\n@include common-account\n"
	got := duoOnlyPAM(input, "michael")
	if strings.Contains(got, "seteuid") || strings.Contains(got, "@include common-auth") || !strings.Contains(got, "auth substack common-auth") {
		t.Fatalf("legacy password/PAM rule remains:\n%s", got)
	}
	if strings.Count(got, pamMarker) != 1 || !strings.Contains(got, "pam_exec.so quiet /usr/lib/proxilens/proxilens-sudo-helper pam-auth") {
		t.Fatalf("correct Duo-only PAM helper rule missing:\n%s", got)
	}
}

func TestValidLocalUsername(t *testing.T) {
	for _, name := range []string{"michael", "_service", "user-1", "user_name"} {
		if !validLocalUsername(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}
	for _, name := range []string{"", "1user", "user name", "user;ALL", "user/name", "-user"} {
		if validLocalUsername(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}

func TestAuthenticationSessionScopePrefersLogindSession(t *testing.T) {
	scope, err := authenticationSessionScope("", "14")
	if err != nil {
		t.Fatal(err)
	}
	if scope != "logind:14" {
		t.Fatalf("unexpected scope %q", scope)
	}
}

func TestAuthenticationSessionScopeRejectsInvalidLogindID(t *testing.T) {
	if _, err := authenticationSessionScope("pts/2", "../../etc"); err == nil {
		t.Fatal("expected invalid logind session ID to be rejected")
	}
}
