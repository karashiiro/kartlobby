package network

import "net"

type UDPConnection struct {
	server *net.UDPConn
	addr   net.Addr
}

var _ Connection = UDPConnection{}

func NewUDPConnection(server *net.UDPConn, addr net.Addr) Connection {
	return &UDPConnection{server: server, addr: addr}
}

func (c UDPConnection) Addr() net.Addr {
	return c.addr
}

func (c UDPConnection) Send(data []byte) error {
	_, err := c.server.WriteTo(data, c.addr)
	return err
}
