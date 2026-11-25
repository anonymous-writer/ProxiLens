package grantbroker

import (
	"os"
	"testing"
	"time"
)

func TestBrokerGrantLifecycleAndIPC(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	b, err := StartServer()
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if got := b.Status(); got.Active {
		t.Fatal("new broker must be inactive")
	}
	expires := b.Start(2 * time.Second)
	if !b.Status().Active {
		t.Fatal("grant should be active")
	}
	remote, err := QueryStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !remote.Active || !remote.ExpiresAt.Equal(expires) {
		t.Fatalf("unexpected IPC status: %#v", remote)
	}
	started, err := StartTestGrant(2 * time.Second)
	if err != nil || !started.Active {
		t.Fatalf("IPC start failed: status=%#v err=%v", started, err)
	}
	revoked, err := RevokeTestGrant()
	if err != nil || revoked.Active {
		t.Fatalf("IPC revoke failed: status=%#v err=%v", revoked, err)
	}
	b.Revoke()
	if b.Status().Active {
		t.Fatal("revoked grant must be inactive")
	}
	info, err := os.Stat(b.socket)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("socket mode = %o, want 600", info.Mode().Perm())
	}
}
