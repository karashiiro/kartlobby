package caching

import "net"

type CachedAddr struct {
	network string
	address string
}

func (c *CachedAddr) Network() string {
	return c.network
}

func (c *CachedAddr) String() string {
	return c.address
}

func NewAddr(addr net.Addr) *CachedAddr {
	return &CachedAddr{
		network: addr.Network(),
		address: addr.String(),
	}
}
