package gateway

import (
	"encoding/json"
	"log"
	"net"

	"github.com/karashiiro/kartlobby/pkg/caching"
	"github.com/karashiiro/kartlobby/pkg/gameinstance"
	"github.com/karashiiro/kartlobby/pkg/network"
)

type gameProxy struct {
	playerAddr   net.Addr
	playerConn   network.Connection
	gameAddr     net.Addr
	gameConn     network.Connection
	proxy        *net.UDPConn
	proxyPort    int
	proxyRunning bool
}

// GameProxyCached represents the data needed to reconstruct a game proxy
// from serialized data.
type GameProxyCached struct {
	PlayerAddr *caching.CachedAddr
	GameAddr   *caching.CachedAddr
	ProxyPort  int
}

func (p *gameProxy) SerializeSelf() ([]byte, error) {
	o := GameProxyCached{
		PlayerAddr: caching.NewAddr(p.playerConn.Addr()),
		GameAddr:   caching.NewAddr(p.gameConn.Addr()),
		ProxyPort:  p.proxyPort,
	}
	return json.Marshal(&o)
}

func (p *gameProxy) DeserializeSelf(data []byte) error {
	o := GameProxyCached{}

	err := json.Unmarshal(data, &o)
	if err != nil {
		return err
	}

	p.playerAddr = o.PlayerAddr
	p.gameAddr = o.GameAddr
	p.proxyPort = o.ProxyPort

	return nil
}

func (p *gameProxy) HydrateDeserialized(gatewayServer *net.UDPConn) error {
	// Restart the UDP server on our proxy port
	proxy, err := net.ListenUDP("udp", &net.UDPAddr{Port: p.proxyPort})
	if err != nil {
		return err
	}

	p.playerConn = network.NewUDPConnection(gatewayServer, p.playerAddr)
	p.gameConn = network.NewUDPConnection(proxy, p.gameAddr)
	p.proxy = proxy

	go p.Run()

	return nil
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
		playerAddr: playerConn.Addr(),
		playerConn: playerConn,
		// Set the remote connection to one that directs messages *from* the
		// proxy server *to* the game
		gameAddr:  inst.Conn.Addr(),
		gameConn:  network.NewUDPConnection(proxy, inst.Conn.Addr()),
		proxy:     proxy,
		proxyPort: proxyPort,
	}, nil
}

func (p *gameProxy) Close() error {
	p.proxyRunning = false
	return p.proxy.Close()
}

func (p *gameProxy) Run() {
	if p.proxyRunning {
		return
	}

	p.proxyRunning = true

	var proxyData [2048]byte
	for p.proxyRunning {
		// Read packet from the game
		n, _, err := p.proxy.ReadFrom(proxyData[:])
		if err != nil {
			if !p.proxyRunning {
				break
			}

			log.Println(err)
			continue
		}

		// Forward packet to the client
		err = p.playerConn.Send(proxyData[:n])
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
