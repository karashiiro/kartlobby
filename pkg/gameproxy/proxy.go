package gameproxy

import (
	"encoding/json"
	"log"
	"net"
	"time"

	"github.com/bep/debounce"
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

// HydrateDeserialized hyhdrates and starts an instance of a game proxy.
// The onClose callback should be used to perform any cleanup that would be
// done after a Close() call by the caller, and it must be provided. Its
// first argument is the player's address.
func (p *GameProxy) HydrateDeserialized(gatewayServer *net.UDPConn, onClose func(string)) error {
	// Restart the UDP server on our proxy port
	proxy, err := net.ListenUDP("udp", &net.UDPAddr{Port: p.proxyPort})
	if err != nil {
		return err
	}

	p.PlayerConn = network.NewUDPConnection(gatewayServer, p.playerAddr)
	p.GameConn = network.NewUDPConnection(proxy, p.gameAddr)
	p.proxy = proxy

	go p.run(onClose)

	return nil
}

// NewGameProxy creates and starts a new instance of a game proxy. The onClose
// callback should be used to perform any cleanup that would be done after a
// Close() call by the caller, and it must be provided. Its first argument is
// the player's address.
func NewGameProxy(playerConn network.Connection, inst *gameinstance.GameInstance, onClose func(string)) (*GameProxy, error) {
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
	go p.run(onClose)

	return p, nil
}

func (p *GameProxy) Close() error {
	p.proxyRunning = false
	return p.proxy.Close()
}

// Run runs the proxy, listening for packets from the game and forwarding them
// to the player. The callback, which takes the player address as its argument,
// must be provided. Any cleanup that an owner of the proxy would normally do
// should be done in the callback.
func (p *GameProxy) run(onClose func(string)) {
	if p.proxyRunning {
		return
	}

	// Timeout setup
	debounced := debounce.New(5 * time.Second)
	onDebounce := func() {
		err := p.Close()
		if err != nil {
			log.Println("debounce:", err)
		}

		onClose(p.playerAddr.String())
	}

	p.proxyRunning = true

	var proxyData [2048]byte
	for p.proxyRunning {
		// Call the timeout debouncer
		debounced(onDebounce)

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
