package gameproxy

import (
	"encoding/json"
	"log"
	"net"

	"github.com/karashiiro/kartlobby/pkg/gameinstance"
	"github.com/karashiiro/kartlobby/pkg/network"
)

type GameProxy struct {
	PlayerConn network.Connection
	GameConn   network.Connection

	playerAddr   net.Addr
	gameAddr     net.Addr
	proxy        *net.UDPConn
	proxyPort    int
	proxyRunning bool
}

// GameProxyCached represents the data needed to reconstruct a game proxy
// from serialized data.
type GameProxyCached struct {
	PlayerAddr *net.UDPAddr
	GameAddr   *net.UDPAddr
	ProxyPort  int
}

func (p *GameProxy) SerializeSelf() ([]byte, error) {
	o := GameProxyCached{
		PlayerAddr: p.PlayerConn.Addr().(*net.UDPAddr),
		GameAddr:   p.GameConn.Addr().(*net.UDPAddr),
		ProxyPort:  p.proxyPort,
	}
	return json.Marshal(&o)
}

func (p *GameProxy) DeserializeSelf(data []byte) error {
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

func (p *GameProxy) HydrateDeserialized(gatewayServer *net.UDPConn) error {
	// Restart the UDP server on our proxy port
	proxy, err := net.ListenUDP("udp", &net.UDPAddr{Port: p.proxyPort})
	if err != nil {
		return err
	}

	p.PlayerConn = network.NewUDPConnection(gatewayServer, p.playerAddr)
	p.GameConn = network.NewUDPConnection(proxy, p.gameAddr)
	p.proxy = proxy

	go p.Run()

	return nil
}

// NewGameProxy creates a new instance of a game proxy.
func NewGameProxy(playerConn network.Connection, inst *gameinstance.GameInstance) (*GameProxy, error) {
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

	p := &GameProxy{
		PlayerConn: playerConn,
		GameConn:   network.NewUDPConnection(proxy, inst.Conn.Addr()),

		playerAddr: playerConn.Addr(),
		// Set the remote connection to one that directs messages *from* the
		// proxy server *to* the game
		gameAddr:  inst.Conn.Addr(),
		proxy:     proxy,
		proxyPort: proxyPort,
	}

	// Run the proxy server
	go p.Run()

	return p, nil
}

func (p *GameProxy) Close() error {
	p.proxyRunning = false
	return p.proxy.Close()
}

func (p *GameProxy) Run() {
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
		err = p.PlayerConn.Send(proxyData[:n])
		if err != nil {
			log.Println(err)
		}
	}
}

func (u *GameProxy) SendToGame(data []byte) error {
	err := u.GameConn.Send(data)
	if err != nil {
		return err
	}

	return nil
}
