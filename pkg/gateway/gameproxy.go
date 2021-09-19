package gateway

import (
	"log"
	"net"

	"github.com/karashiiro/kartlobby/pkg/gameinstance"
	"github.com/karashiiro/kartlobby/pkg/network"
)

type gameProxy struct {
	playerConn   network.Connection
	gameConn     network.Connection
	proxy        *net.UDPConn
	proxyRunning bool
}

func newGameProxy(playerConn network.Connection, inst *gameinstance.GameInstance) (*gameProxy, error) {
	// Get a free port to proxy through
	proxyPort, err := network.GetFreePort()
	if err != nil {
		return nil, err
	}

	// Start a new UDP server on our proxy port
	proxy, err := net.ListenUDP("udp", &net.UDPAddr{Port: proxyPort})
	if err != nil {
		return nil, err
	}

	return &gameProxy{
		playerConn: playerConn,
		// Set the remote connection to one that directs messages *from* the
		// proxy server *to* the game
		gameConn: network.NewUDPConnection(proxy, inst.Conn.Addr()),
		proxy:    proxy,
	}, nil
}

func (u *gameProxy) Close() error {
	u.proxyRunning = false
	return u.proxy.Close()
}

func (u *gameProxy) Run() {
	u.proxyRunning = true

	var proxyData [2048]byte
	for u.proxyRunning {
		// Read packet from the game
		n, _, err := u.proxy.ReadFrom(proxyData[:])
		if err != nil {
			if !u.proxyRunning {
				break
			}

			log.Println(err)
			continue
		}

		// Forward packet to the client
		err = u.playerConn.Send(proxyData[:n])
		if err != nil {
			log.Println(err)
		}
	}
}

func (u *gameProxy) SendToGame(data []byte) error {
	err := u.gameConn.Send(data)
	if err != nil {
		return err
	}

	return nil
}
