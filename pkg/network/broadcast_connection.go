package network

import (
	"errors"
	"net"
)

type BroadcastConnection struct {
	conns    []Connection
	numConns int
	maxConns int
}

var _ Connection = BroadcastConnection{}

func NewBroadcastConnection(size int) *BroadcastConnection {
	b := &BroadcastConnection{
		conns:    make([]Connection, size),
		numConns: 0,
		maxConns: size,
	}
	return b
}

func (b BroadcastConnection) Addr() net.Addr {
	return nil
}

func (b BroadcastConnection) Send(data []byte) error {
	for i := 0; i < b.numConns; i++ {
		// Ignore error, we should log this
		_ = b.conns[i].Send(data)
	}

	return nil
}

func (b *BroadcastConnection) Set(conn Connection) error {
	if b.numConns >= b.maxConns {
		return errors.New("maximum connection count reached")
	}

	// TODO: fix race condition
	b.conns[b.numConns] = conn
	b.numConns++

	return nil
}

func (b *BroadcastConnection) Unset(conn Connection) {
	connIdx := -1
	for i := 0; i < len(b.conns); i++ {
		if b.conns[i].Addr().String() == conn.Addr().String() {
			connIdx = i
		}
	}

	if connIdx == -1 {
		return
	}

	// TODO: fix race condition
	b.conns[connIdx] = b.conns[b.numConns-1]
	b.conns[b.numConns-1] = nil
	b.numConns--
}
