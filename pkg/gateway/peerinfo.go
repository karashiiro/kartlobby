package gateway

import (
	"net"

	"github.com/karashiiro/kartlobby/pkg/network"
)

type peerInfo struct {
	localAddr  net.Addr
	remoteConn network.Connection
	proxy      *net.UDPConn
}
