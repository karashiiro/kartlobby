package gateway

import (
	"net"

	"github.com/karashiiro/kartlobby/pkg/network"
)

type clientInfo struct {
	clientAddr net.Addr
	gameConn   network.Connection
	proxy      *net.UDPConn
}
