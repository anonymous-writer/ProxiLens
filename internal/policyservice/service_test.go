package policyservice

import (
	"strings"
	"testing"
)

func TestReadBoundedLineAcceptsOneFrame(t *testing.T) {
	got, err := readBoundedLine(strings.NewReader("abc\n"), 4)
	if err != nil || got != "abc\n" {
		t.Fatalf("got %q, err %v", got, err)
	}
}

func TestReadBoundedLineRejectsOversize(t *testing.T) {
	if _, err := readBoundedLine(strings.NewReader("abcdef\n"), 4); err == nil {
		t.Fatal("expected oversized request rejection")
	}
}

func TestReadBoundedLineRejectsMissingNewline(t *testing.T) {
	if _, err := readBoundedLine(strings.NewReader("abc"), 4); err == nil {
		t.Fatal("expected unterminated request rejection")
	}
}

func TestReadBoundedLineRejectsTrailingFrames(t *testing.T) {
	for _, input := range []string{"abc\nxyz", "abc\n\n"} {
		if _, err := readBoundedLine(strings.NewReader(input), 32); err == nil {
			t.Fatalf("expected trailing data rejection for %q", input)
		}
	}
}

func TestHasSessionScopeRequiresExactComponent(t *testing.T) {
	if !hasSessionScope("0::/user.slice/user-1000.slice/session-1.scope", "1") {
		t.Fatal("expected exact session scope match")
	}
	if hasSessionScope("0::/user.slice/session-10.scope", "1") {
		t.Fatal("partial session ID must not match")
	}
}

func TestProcessStartTime(t *testing.T) {
	if _, err := processStartTime(1); err != nil {
		t.Fatalf("read own test process identity: %v", err)
	}
}
