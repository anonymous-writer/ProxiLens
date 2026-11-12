package duo

import (
	"net/url"
	"testing"
)

func TestCanonicalQueryUsesRFC3986Encoding(t *testing.T) {
	got := canonicalQuery(url.Values{
		"realname": {"First Last"},
		"username": {"michael~test"},
	})
	want := "realname=First%20Last&username=michael~test"
	if got != want {
		t.Fatalf("canonicalQuery() = %q, want %q", got, want)
	}
}
