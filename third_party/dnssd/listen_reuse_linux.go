//go:build linux

package dnssd

import (
	"context"
	"fmt"
	"net"
	"syscall"
)

// listenMDNSUDP enables address reuse before bind so this responder can
// coexist with the stock mDNS/HomeKit service on embedded Linux gateways.
func listenMDNSUDP(network string, address *net.UDPAddr) (*net.UDPConn, error) {
	config := net.ListenConfig{
		Control: func(_, _ string, raw syscall.RawConn) error {
			var socketErr error
			if err := raw.Control(func(fd uintptr) {
				socketErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				if socketErr == nil {
					// SO_REUSEPORT is optional on older kernels. SO_REUSEADDR is the
					// required option for sharing an mDNS multicast listener.
					_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
				}
			}); err != nil {
				return err
			}
			return socketErr
		},
	}
	packetConn, err := config.ListenPacket(context.Background(), network, address.String())
	if err != nil {
		return nil, err
	}
	udpConn, ok := packetConn.(*net.UDPConn)
	if !ok {
		_ = packetConn.Close()
		return nil, fmt.Errorf("unexpected packet connection type %T", packetConn)
	}
	return udpConn, nil
}
