package gameinstance

import (
	"context"
	"errors"
	"log"
	"math"
	"net"
	"time"

	"github.com/karashiiro/kartlobby/pkg/doom"
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

func (m *GameInstanceManager) IsInstanceAddress(addr net.Addr) bool {
	for instAddr, _ := range m.instances {
		if instAddr == addr.String() {
			return true
		}
	}

	return false
}

// AskInfo sends a PT_ASKINFO request to the game server behind the first available instance we're tracking,
// returning the resulting PT_SERVERINFO and PT_PLAYERINFO packets. A timeout context should
// always be provided in order to prevent an application hang in the event that the server doesn't respond.
func (m *GameInstanceManager) AskInfo(askInfo *gamenet.AskInfoPak, server UDPServer, ctx context.Context) (*gamenet.ServerInfoPak, *gamenet.PlayerInfoPak, error) {
	for _, inst := range m.instances {
		si, pi, err := inst.AskInfo(askInfo, server, ctx)
		if err == nil {
			return si, pi, nil
		}
	}

	return nil, nil, errors.New("no instances are active")
}

// CreateInstance creates a new instance, returning an error if this fails for any reason.
func (m *GameInstanceManager) CreateInstance(conn *net.UDPConn) (*GameInstance, error) {
	// Create a new instance
	newInstance, err := newInstance(conn)
	if err != nil {
		return nil, err
	}

	// Register the instance
	m.instances[newInstance.Conn.Addr().String()] = newInstance

	log.Printf("Created new instance %s on port %d", newInstance.id, newInstance.port)

	return newInstance, nil
}

// GetOrCreateOpenInstance gets an open game instance, preferring instances with fewer players
// in order to balance players across all instances. In the event that this isn't possible, a
// new instance will be created. If we are already tracking our maximum number of instances,
// an error is returned.
func (m *GameInstanceManager) GetOrCreateOpenInstance(conn *net.UDPConn, server UDPServer) (*GameInstance, error) {
	var instancePlayers int = math.MaxInt
	var instance *GameInstance

	for _, inst := range m.instances {
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		// Get the player info from the server
		_, pi, err := inst.AskInfo(&gamenet.AskInfoPak{
			PacketHeader: gamenet.PacketHeader{
				PacketType: gamenet.PT_ASKINFO,
			},
			Version: doom.VERSION,
			Time:    uint32(time.Now().Unix()),
		}, server, ctx)
		if err != nil {
			// We should do some recovery or cleanup here, maybe?
			log.Println(err)
			continue
		}

		// Check the number of players
		instPlayers := 0
		for i := 0; i < len(pi.Players); i++ {
			if pi.Players[i].Node == 255 {
				break
			}

			instPlayers++
		}

		// We want the instance with the fewest players
		if instance == nil || instPlayers < instancePlayers {
			instance = inst
			instancePlayers = instPlayers
		}
	}

	if instance == nil {
		// Create a new instance
		newInstance, err := m.CreateInstance(conn)
		if err != nil {
			return nil, err
		}

		// Assign it to return it
		instance = newInstance
	}

	return instance, nil
}

// GetInstance returns the instance with the specified address, or an error if that instance
// is not registered with this manager.
func (m *GameInstanceManager) GetInstance(addr string) (*GameInstance, error) {
	if inst, ok := m.instances[addr]; ok {
		return inst, nil
	}

	return nil, errors.New("instance is not registered")
}
