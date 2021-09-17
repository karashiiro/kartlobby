package network

import "net"

type Connection interface {
	Addr() net.Addr
	Send(data []byte) error
}
