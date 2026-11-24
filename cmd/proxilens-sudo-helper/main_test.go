//go:build linux

package main

import (
	"strings"
	"testing"
)

func TestPAMRuleIsInsertedImmediatelyAfterCommonAuth(t *testing.T) {
	input := "auth sufficient pam_rootok.so\n@include common-auth\n@include common-account\n"
	got := addPAMLine(input)
	want := "auth sufficient pam_rootok.so\n@include common-auth\n" + pamMarker + "\n" + pamLine[strings.Index(pamLine, "\n")+1:] + "\n@include common-account\n"
	if got != want {
		t.Fatalf("unexpected PAM configuration:\n%s", got)
	}
	if again := addPAMLine(got); again != got {
		t.Fatal("PAM rule was inserted more than once")
	}
}

func TestPAMRuleRemovalPreservesOtherRules(t *testing.T) {
	input := "auth sufficient pam_rootok.so\n@include common-auth\n" + pamLine + "\n@include common-account\n"
	got := removePAMLine(input)
	want := "auth sufficient pam_rootok.so\n@include common-auth\n@include common-account\n"
	if got != want {
		t.Fatalf("unexpected PAM configuration after removal:\n%s", got)
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

func TestPAMRuleUpgradeReplacesLegacySeteuidRule(t *testing.T) {
	input := "@include common-auth\n" + pamMarker + "\nauth required pam_exec.so quiet seteuid /usr/lib/proxilens/proxilens-sudo-helper pam-auth\n@include common-account\n"
	got := addPAMLine(input)
	if strings.Contains(got, "seteuid") {
		t.Fatalf("legacy seteuid option remains in PAM rule:\n%s", got)
	}
	if strings.Count(got, pamMarker) != 1 {
		t.Fatalf("expected exactly one ProxiLens PAM marker:\n%s", got)
	}
	if !strings.Contains(got, "pam_exec.so quiet /usr/lib/proxilens/proxilens-sudo-helper pam-auth") {
		t.Fatalf("corrected PAM helper rule missing:\n%s", got)
	}
}
