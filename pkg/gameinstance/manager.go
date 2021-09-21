package gameinstance

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"github.com/karashiiro/kartlobby/pkg/doom"
	"github.com/karashiiro/kartlobby/pkg/gamenet"
)

type GameInstanceManagerOptions struct {
	MaxInstances   int
	DockerImage    string
	GameConfigPath string
	GameAddonPath  string
}

type GameInstanceManager struct {
	numInstances            int
	maxInstances            int
	instances               map[string]*GameInstance
	instanceGetOrCreateLock *sync.Mutex
	instanceCreateLock      *sync.Mutex
	client                  *client.Client
	reaperRunning           bool
	image                   string
	configPath              string
	addonPath               string
}

// GameInstanceManagerCached represents the least information
// needed to restore the state of a full GameInstanceManager.
type GameInstanceManagerCached struct {
	Instances map[string]string
}

func (m *GameInstanceManager) SerializeSelf() ([]byte, error) {
	cached := GameInstanceManagerCached{
		Instances: make(map[string]string),
	}

	for addr, inst := range m.instances {
		instSerialized, err := inst.SerializeSelf()
		if err != nil {
			return nil, err
		}

		cached.Instances[addr] = string(instSerialized)
	}

	return json.Marshal(&cached)
}

func (m *GameInstanceManager) DeserializeSelf(data []byte) error {
	o := GameInstanceManagerCached{}

	err := json.Unmarshal(data, &o)
	if err != nil {
		return err
	}

	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	m.instances = make(map[string]*GameInstance)
	for addr, instSerialized := range o.Instances {
		inst := &GameInstance{}
		err := inst.DeserializeSelf([]byte(instSerialized))
		if err != nil {
			return err
		}

		m.instances[addr] = inst
	}

	m.instanceGetOrCreateLock = &sync.Mutex{}
	m.instanceCreateLock = &sync.Mutex{}
	m.client = client

	return nil
}

func (m *GameInstanceManager) HydrateDeserialized(server *net.UDPConn, image, configPath, addonPath string) {
	m.image = image
	m.configPath = configPath
	m.addonPath = addonPath
	for _, inst := range m.instances {
		inst.HydrateDeserialized(m.client, server)
	}
}

// NewManager creates a new game instance manager. The maxInstances parameter controls
// how many rooms may exist at one time. Setting this to -1 (not recommended) will
// effectively uncap the instance limit.
func NewManager(opts *GameInstanceManagerOptions) (*GameInstanceManager, error) {
	maxInstances := opts.MaxInstances
	if maxInstances == -1 {
		maxInstances = math.MaxInt
	}

	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	m := GameInstanceManager{
		numInstances:            0,
		maxInstances:            maxInstances,
		instances:               make(map[string]*GameInstance),
		instanceGetOrCreateLock: &sync.Mutex{},
		instanceCreateLock:      &sync.Mutex{},
		client:                  client,
		reaperRunning:           false,
		image:                   opts.DockerImage,
		configPath:              opts.GameConfigPath,
		addonPath:               opts.GameAddonPath,
	}

	return &m, nil
}

// Reaper runs a blocking loop that cleans up dead instances. A function may be
// optionally provided to run a callback when an instance is stopped. This callback
// takes the server's address as its first argument.
func (m *GameInstanceManager) Reaper(server UDPServer, stopFn func(string)) {
	if m.reaperRunning {
		return
	}

	m.reaperRunning = true
	for m.reaperRunning {
		instancesToRemove := make([]string, 0)

		for addr, inst := range m.instances {
			if !m.reaperRunning {
				break
			}

			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, 3*time.Second)

			// Check if the instance should be stopped
			shouldClose, err := inst.ShouldClose(server, ctx)
			if err != nil {
				log.Println(err)
				cancel()
				continue
			}

			// Close and remove the instance if it should be stopped
			if shouldClose {
				err := inst.Stop()
				if err != nil {
					log.Println(err)
					cancel()
					continue
				}

				if stopFn != nil {
					stopFn(addr)
				}

				// Add this to our list to remove from the map
				instancesToRemove = append(instancesToRemove, addr)

				log.Printf("Stopped container %s", inst.id)
			}

			cancel()
		}

		// Remove stopped instances from our map
		m.instanceCreateLock.Lock()
		for _, addr := range instancesToRemove {
			delete(m.instances, addr)
			m.numInstances--
		}
		m.instanceCreateLock.Unlock()

		time.Sleep(3 * time.Second)
	}
}

// Close stops the reaper loop.
func (m *GameInstanceManager) Close() {
	m.reaperRunning = false
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

// CreateInstance creates a new instance, returning an error if this fails for any reason, including the
// maximum number of instances already having been created.
func (m *GameInstanceManager) CreateInstance(conn *net.UDPConn) (*GameInstance, error) {
	// Lock here so that concurrent requests don't risk pushing
	// us over the maximum instance count
	m.instanceCreateLock.Lock()
	defer m.instanceCreateLock.Unlock()

	// Check if we've maxed-out on instances
	if m.numInstances == m.maxInstances {
		return nil, errors.New("maximum number of instances created")
	}

	// Create a new instance
	newInstance, err := newInstance(m.client, conn, m.image, m.configPath, m.addonPath)
	if err != nil {
		return nil, err
	}

	// Register the instance
	m.instances[newInstance.Conn.Addr().String()] = newInstance

	log.Printf("Created new instance %s on port %d", newInstance.id, newInstance.port)
	m.numInstances++

	return newInstance, nil
}

// GetOrCreateOpenInstance gets an open game instance, preferring instances with fewer players
// in order to balance players across all instances. In the event that this isn't possible, a
// new instance will be created. If we are already tracking our maximum number of instances,
// an error is returned.
func (m *GameInstanceManager) GetOrCreateOpenInstance(conn *net.UDPConn, server UDPServer) (*GameInstance, error) {
	// We lock here so that if another get/create request occurs that results in an instance being created,
	// that happens before we attempt to do the same ourselves. Otherwise, many connections occurring at once
	// could create a bunch of containers and overload the server.
	m.instanceGetOrCreateLock.Lock()
	defer m.instanceGetOrCreateLock.Unlock()

	var instancePlayers int = math.MaxInt
	var instance *GameInstance

	for _, inst := range m.instances {
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		// Get the player info from the server
		si, _, err := inst.AskInfo(&gamenet.AskInfoPak{
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

		// We want the instance with the fewest players
		if instance == nil || int(si.NumberOfPlayer) < instancePlayers {
			instance = inst
			instancePlayers = int(si.NumberOfPlayer)
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
