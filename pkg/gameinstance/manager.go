package gameinstance

import "errors"

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

func (m *GameInstanceManager) GetOrCreateOpenInstance() (*GameInstance, error) {
	return nil, nil
}

func (m *GameInstanceManager) GetInstance(addr string) (*GameInstance, error) {
	if inst, ok := m.instances[addr]; ok {
		return inst, nil
	}

	return nil, errors.New("instance is not registered")
}
