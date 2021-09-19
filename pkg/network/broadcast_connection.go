package network

import (
	"errors"
	"net"
	"sync"
)

type BroadcastConnection struct {
	conns    []Connection
	setMutex *sync.Mutex
	numConns int
}

var _ Connection = BroadcastConnection{}

func NewBroadcastConnection(initialSlots int) (*BroadcastConnection, error) {
	if initialSlots == 0 {
		return nil, errors.New("initialSlots must be greater than 0")
	}

	b := &BroadcastConnection{
		conns:    make([]Connection, initialSlots),
		setMutex: &sync.Mutex{},
		numConns: 0,
	}

	return b, nil
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
	// If the maximum connection count has been reached
	if b.numConns == cap(b.conns) {
		// Double the connections slice
		b.conns = append(b.conns, make([]Connection, b.numConns)...)
	}

	b.setMutex.Lock()
	defer b.setMutex.Unlock()
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

	b.setMutex.Lock()
	defer b.setMutex.Unlock()
	b.conns[connIdx] = b.conns[b.numConns-1]
	b.conns[b.numConns-1] = nil
	b.numConns--
}
