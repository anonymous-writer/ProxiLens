//go:build linux

package policyproto

import (
	"errors"
	"net"
	"runtime"
	"syscall"
)

// KernelPeerCredentials reads credentials attached by the Linux kernel to an
// accepted Unix-domain socket. Call it on the server-side connection. The
// returned PID/UID/GID are kernel supplied; SessionID must still be resolved
// independently from logind using PID/UID before MatchPeer is called.
type KernelPeerCredentials struct {
	PID int32
	UID uint32
	GID uint32
}

func ReadKernelPeerCredentials(conn *net.UnixConn) (KernelPeerCredentials, error) {
	if conn == nil {
		return KernelPeerCredentials{}, errors.New("nil Unix connection")
	}
	raw, err := conn.SyscallConn()
	if err != nil {
		return KernelPeerCredentials{}, err
	}
	var cred *syscall.Ucred
	var sockErr error
	err = raw.Control(func(fd uintptr) {
		cred, sockErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	runtime.KeepAlive(conn)
	if err != nil {
		return KernelPeerCredentials{}, err
	}
	if sockErr != nil {
		return KernelPeerCredentials{}, sockErr
	}
	if cred == nil {
		return KernelPeerCredentials{}, errors.New("kernel returned no peer credentials")
	}
	return KernelPeerCredentials{PID: cred.Pid, UID: cred.Uid, GID: cred.Gid}, nil
}
