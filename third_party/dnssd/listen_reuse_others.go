//go:build !linux

package dnssd

import "net"

func listenMDNSUDP(network string, address *net.UDPAddr) (*net.UDPConn, error) {
	return net.ListenUDP(network, address)
}
