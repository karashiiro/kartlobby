package gameinstance

import "errors"

type GameInstanceManager struct {
	numInstances int
	maxInstances int
	instances    map[string]string
}

func NewManager(maxInstances int) *GameInstanceManager {
	m := GameInstanceManager{
		numInstances: 0,
		maxInstances: maxInstances,
		instances:    make(map[string]string),
	}

	return &m
}

func (m *GameInstanceManager) GetOrCreateOpenInstance() (uint32, error) {
	return 0, nil
}

func (m *GameInstanceManager) GetInstance(addr string) (string, error) {
	if inst, ok := m.instances[addr]; ok {
		return inst, nil
	}

	return "", errors.New("instance is not registered")
}
