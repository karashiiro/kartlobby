package gameinstance

import (
	"context"
	"errors"

	"github.com/karashiiro/kartlobby/pkg/gamenet"
)

type GameInstanceManager struct {
	numInstances int
	maxInstances int
	instances    map[string]*GameInstance
}

func NewManager(maxInstances int) *GameInstanceManager {
	m := GameInstanceManager{
		numInstances: 0,
		maxInstances: maxInstances,
		instances:    make(map[string]*GameInstance),
	}

	return &m
}

// AskInfo sends a PT_ASKINFO request to the game server behind the first instance we're tracking,
// returning the resulting PT_SERVERINFO and PT_PLAYERINFO packets. A timeout context should
// always be provided in order to prevent an application hang in the event that the server doesn't respond.
func (m *GameInstanceManager) AskInfo(server UDPServer, ctx context.Context) (*gamenet.ServerInfoPak, *gamenet.PlayerInfoPak, error) {
	// Get first instance
	var instance *GameInstance
	for _, inst := range m.instances {
		instance = inst
		break
	}

	if instance == nil {
		return nil, nil, errors.New("no instances are active")
	}

	return instance.AskInfo(server, ctx)
}

// GetOrCreateOpenInstance gets an open game instance, preferring instances with fewer players
// in order to balance players across all instances. In the event that this isn't possible, a
// new instance will be created. If we are already tracking our maximum number of instances,
// an error is returned.
func (m *GameInstanceManager) GetOrCreateOpenInstance() (*GameInstance, error) {
	return nil, nil
}

// GetInstance returns the instance with the specified address, or an error if that instance
// is not registered with this manager.
func (m *GameInstanceManager) GetInstance(addr string) (*GameInstance, error) {
	if inst, ok := m.instances[addr]; ok {
		return inst, nil
	}

	return nil, errors.New("instance is not registered")
}
