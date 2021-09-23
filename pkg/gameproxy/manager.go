package gameproxy

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"sync"

	"github.com/karashiiro/kartlobby/pkg/gameinstance"
	"github.com/karashiiro/kartlobby/pkg/network"
)

type GameProxyManager struct {
	// Connection map from clients to servers
	clients      map[string]*GameProxy
	clientsMutex *sync.Mutex
}

type GameProxyManagerCached struct {
	Clients map[string]string
}

func (m *GameProxyManager) SerializeSelf() ([]byte, error) {
	o := GameProxyManagerCached{}

	o.Clients = make(map[string]string)
	for addr, proxy := range m.clients {
		proxySerialized, err := proxy.SerializeSelf()
		if err != nil {
			return nil, err
		}

		o.Clients[addr] = string(proxySerialized)
	}

	return json.Marshal(&o)
}

func (m *GameProxyManager) DeserializeSelf(data []byte) error {
	o := GameProxyManagerCached{}
	err := json.Unmarshal(data, &o)
	if err != nil {
		return err
	}

	m.clients = make(map[string]*GameProxy)
	for addr, proxySerialized := range o.Clients {
		proxy := &GameProxy{}
		err := proxy.DeserializeSelf([]byte(proxySerialized))
		if err != nil {
			return err
		}

		m.clients[addr] = proxy
	}

	m.clientsMutex = &sync.Mutex{}

	return nil
}

func (m *GameProxyManager) HydrateDeserialized(gatewayServer *net.UDPConn) error {
	for _, proxy := range m.clients {
		err := proxy.HydrateDeserialized(gatewayServer, m.onProxyDie)
		if err != nil {
			return err
		}
	}

	return nil
}

// NewProxyManager returns a new game proxy manager.
func NewGameProxyManager() *GameProxyManager {
	return &GameProxyManager{
		clients:      make(map[string]*GameProxy),
		clientsMutex: &sync.Mutex{},
	}
}

func (m *GameProxyManager) GetProxy(addr string) (*GameProxy, error) {
	if gp, ok := m.clients[addr]; ok {
		return gp, nil
	}

	return nil, errors.New("no proxy matches the provided address")
}

// CreateProxy creates a proxy from the provided player connection to the specified instance.
func (m *GameProxyManager) CreateProxy(playerConn network.Connection, inst *gameinstance.GameInstance) (*GameProxy, error) {
	m.clientsMutex.Lock()
	defer m.clientsMutex.Unlock()

	// Start a new UDP server to proxy through
	p, err := NewGameProxy(playerConn, inst, m.onProxyDie)
	if err != nil {
		return nil, err
	}

	addr := playerConn.Addr().String()

	m.clients[addr] = p

	return p, nil
}

func (m *GameProxyManager) onProxyDie(addr string) {
	m.clientsMutex.Lock()
	defer m.clientsMutex.Unlock()
	delete(m.clients, addr)
	log.Printf("Proxy %s removed", addr)
}

// RemoveConnectionsTo removes all connections to the provided address. A callback
// may be provided that will be called for each stopped proxy.
func (m *GameProxyManager) RemoveConnectionsTo(addr string, cb func(network.Connection)) error {
	m.clientsMutex.Lock()
	defer m.clientsMutex.Unlock()

	clientsToRemove := make(map[string]*GameProxy)

	// Stop any connections involving the stopped instance
	for cAddr, proxy := range m.clients {
		if proxy.GameConn.Addr().String() == addr {
			err := proxy.Close()
			if err != nil {
				return err
			}

			clientsToRemove[cAddr] = proxy
		}
	}

	// Remove all connections we stopped from our proxy map
	for cAddr, proxy := range clientsToRemove {
		delete(m.clients, cAddr)
		cb(proxy.PlayerConn)
	}

	return nil
}
