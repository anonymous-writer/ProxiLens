//go:build linux

package policyproto

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadKernelPeerCredentialsFromAcceptedSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peer.sock")
	addr := &net.UnixAddr{Name: path, Net: "unix"}
	ln, err := net.ListenUnix("unix", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	accepted := make(chan *net.UnixConn, 1)
	go func() {
		c, e := ln.AcceptUnix()
		if e == nil {
			accepted <- c
		}
	}()
	client, err := net.DialUnix("unix", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var server *net.UnixConn
	select {
	case server = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("accept timeout")
	}
	defer server.Close()
	got, err := ReadKernelPeerCredentials(server)
	if err != nil {
		t.Fatal(err)
	}
	if got.PID <= 0 {
		t.Fatalf("invalid peer PID: %d", got.PID)
	}
	if got.UID != uint32(os.Getuid()) {
		t.Fatalf("peer UID=%d, want %d", got.UID, os.Getuid())
	}
}
