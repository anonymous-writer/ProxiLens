//go:build !linux

package policyproto

import (
	"errors"
	"net"
)

type KernelPeerCredentials struct {
	PID int32
	UID uint32
	GID uint32
}

func ReadKernelPeerCredentials(conn *net.UnixConn) (KernelPeerCredentials, error) {
	return KernelPeerCredentials{}, errors.New("kernel Unix peer credentials are only implemented on Linux")
}
