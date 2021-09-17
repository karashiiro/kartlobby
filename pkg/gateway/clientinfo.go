package gateway

import (
	"github.com/karashiiro/kartlobby/pkg/network"
)

type clientInfo struct {
	clientConn network.Connection
	gameConn   network.Connection
}
