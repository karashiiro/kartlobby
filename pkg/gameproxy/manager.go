package gameproxy

import (
	"errors"
	"sync"

	"github.com/karashiiro/kartlobby/pkg/gameinstance"
	"github.com/karashiiro/kartlobby/pkg/network"
)

type GameProxyManager struct {
	// Connection map from clients to servers
	clients      map[string]*GameProxy
	clientsMutex *sync.Mutex
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

func (m *GameProxyManager) CreateProxy(playerConn network.Connection, inst *gameinstance.GameInstance, addr string) (*GameProxy, error) {
	m.clientsMutex.Lock()
	defer m.clientsMutex.Unlock()

	// Start a new UDP server to proxy through
	p, err := NewGameProxy(playerConn, inst)
	if err != nil {
		return nil, err
	}

	m.clients[addr] = p

	return p, nil
}

func (m *GameProxyManager) RemoveConnectionsTo(addr string) error {
	m.clientsMutex.Lock()
	defer m.clientsMutex.Unlock()

	clientsToRemove := make([]string, 0)

	// Stop any connections involving the stopped instance
	for cAddr, proxy := range m.clients {
		if proxy.GameConn.Addr().String() == addr {
			err := proxy.Close()
			if err != nil {
				return err
			}

			clientsToRemove = append(clientsToRemove, cAddr)
		}
	}

	// Remove all connections we stopped from our proxy map
	for _, cAddr := range clientsToRemove {
		delete(m.clients, cAddr)
	}

	return nil
}
