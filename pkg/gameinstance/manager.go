package gameinstance

import (
	"context"
	"errors"
	"time"

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

func (m *GameInstanceManager) AskInfo(server UDPServer) (*gamenet.ServerInfoPak, *gamenet.PlayerInfoPak, error) {
	if len(m.instances) == 0 {
		return nil, nil, errors.New("no instances are active")
	}

	var instance *GameInstance
	for _, inst := range m.instances {
		instance = inst
		break
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	return instance.AskInfo(server, ctx)
}

func (m *GameInstanceManager) GetOrCreateOpenInstance() (*GameInstance, error) {
	return nil, nil
}

func (m *GameInstanceManager) GetInstance(addr string) (*GameInstance, error) {
	if inst, ok := m.instances[addr]; ok {
		return inst, nil
	}

	return nil, errors.New("instance is not registered")
}
