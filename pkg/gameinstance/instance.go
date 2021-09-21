package gameinstance

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/karashiiro/kartlobby/pkg/doom"
	"github.com/karashiiro/kartlobby/pkg/gamenet"
	"github.com/karashiiro/kartlobby/pkg/network"
)

type GameInstance struct {
	Conn network.Connection

	client *client.Client
	id     string
	port   int
	addr   net.Addr
}

// GameInstanceCached represents the least information
// needed to reconstruct the state of a full GameInstance.
type GameInstanceCached struct {
	ID   string
	Port int
	Addr *net.UDPAddr
}

func (gi *GameInstance) SerializeSelf() ([]byte, error) {
	cached := GameInstanceCached{
		ID:   gi.id,
		Port: gi.port,
		Addr: gi.Conn.Addr().(*net.UDPAddr),
	}

	return json.Marshal(&cached)
}

func (gi *GameInstance) DeserializeSelf(data []byte) error {
	o := GameInstanceCached{}

	err := json.Unmarshal(data, &o)
	if err != nil {
		return err
	}

	gi.id = o.ID
	gi.port = o.Port
	gi.addr = o.Addr

	return nil
}

func (gi *GameInstance) HydrateDeserialized(client *client.Client, server *net.UDPConn) {
	gi.Conn = network.NewUDPConnection(server, gi.addr)
	gi.client = client
}

func newInstance(client *client.Client, server *net.UDPConn, image string, configPath string, addonPath string) (*GameInstance, error) {
	ctx := context.Background()

	// Get a free port
	port, err := network.GetFreePort()
	if err != nil {
		return nil, err
	}

	// Set up volume mapping
	mounts := make([]mount.Mount, 0)
	if configPath != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: configPath,
			Target: "/config",
		})
	}

	if addonPath != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: addonPath,
			Target: "/addons",
		})
	}

	// Create the container
	resp, err := client.ContainerCreate(ctx, &container.Config{
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Image:        image,
	}, &container.HostConfig{
		PortBindings: nat.PortMap{
			// Bind 5029/udp in the container to our free port on the host
			nat.Port("5029/udp"): []nat.PortBinding{{
				HostPort: fmt.Sprint(port),
			}},
		},
		Mounts: mounts,
	}, nil, nil, "")
	if err != nil {
		return nil, err
	}

	// Start the container
	if err := client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	// Follow the logs and wait until the server is ready
	reader, err := client.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
		ShowStderr: true,
		ShowStdout: true,
		Timestamps: false,
		Follow:     true,
	})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	got := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			logLine := scanner.Text()
			if logLine == "Entering main game loop..." {
				got <- true
				break
			}
		}
	}()
	go func() {
		<-ctx.Done()
		got <- false
	}()

	if <-got && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Get the host IP
	ip, err := network.GetLocalIP()
	if err != nil {
		return nil, err
	}

	addr := &net.UDPAddr{
		IP:   *ip,
		Port: port,
	}

	inst := &GameInstance{
		Conn: network.NewUDPConnection(server, addr),

		client: client,
		id:     resp.ID,
		port:   port,
		addr:   addr,
	}

	return inst, nil
}

func (gi *GameInstance) GetID() string {
	return gi.id
}

func (gi *GameInstance) GetPort() int {
	return gi.port
}

// AskInfo sends a PT_ASKINFO request to the game server behind this instance, returning the resulting
// PT_SERVERINFO and PT_PLAYERINFO packets. A timeout context should always be provided in order to
// prevent an application hang in the event that the server doesn't respond.
func (gi *GameInstance) AskInfo(askInfo *gamenet.AskInfoPak, server UDPServer, ctx context.Context) (*gamenet.ServerInfoPak, *gamenet.PlayerInfoPak, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	serverInfoChan := make(chan []byte, 1)
	playerInfoChan := make(chan []byte, 1)

	siErrChan := make(chan error, 1)
	piErrChan := make(chan error, 1)

	// Launch goroutines waiting for the responses
	go server.WaitForInstanceMessage(&UDPCallbackKey{
		Context:  uuid.New().String(),
		GamePort: gi.port,
		Message:  gamenet.PT_SERVERINFO,
	}, serverInfoChan, siErrChan, ctx)
	go server.WaitForInstanceMessage(&UDPCallbackKey{
		Context:  uuid.New().String(),
		GamePort: gi.port,
		Message:  gamenet.PT_PLAYERINFO,
	}, playerInfoChan, piErrChan, ctx)

	// Forward the PT_ASKINFO request from the caller
	err := gamenet.SendPacket(gi.Conn, askInfo)
	if err != nil {
		return nil, nil, err
	}

	// Wait for a result
	serverInfoBytes := <-serverInfoChan
	playerInfoBytes := <-playerInfoChan

	// Error checking; context cancellations
	siErr := <-siErrChan
	if siErr != nil {
		return nil, nil, siErr
	}

	piErr := <-piErrChan
	if piErr != nil {
		return nil, nil, piErr
	}

	// Unmarshalling and returning results
	serverInfo := gamenet.ServerInfoPak{}
	playerInfo := gamenet.PlayerInfoPak{}

	err = gamenet.ReadPacket(serverInfoBytes, &serverInfo)
	if err != nil {
		return nil, nil, err
	}

	err = gamenet.ReadPacket(playerInfoBytes, &playerInfo)
	if err != nil {
		return nil, nil, err
	}

	return &serverInfo, &playerInfo, nil
}

// ShouldClose returns true if the instance has been active for more than a few minutes
// and has no players. A timeout context should always be provided in order to
// prevent an application hang in the event that the server doesn't respond.
func (gi *GameInstance) ShouldClose(server UDPServer, ctx context.Context) (bool, error) {
	inspect, err := gi.client.ContainerInspect(ctx, gi.id)
	if err != nil {
		// This is probably something we should be aware of, return the error
		return false, err
	}

	// Parse the creation time
	creationTime, err := time.Parse(time.RFC3339, inspect.Created)
	if err != nil {
		return false, err
	}

	if !creationTime.Before(time.Now().Add(-2 * time.Minute)) {
		// This is a new instance
		return false, nil
	}

	// Get the instance's player info
	si, _, err := gi.AskInfo(&gamenet.AskInfoPak{
		PacketHeader: gamenet.PacketHeader{
			PacketType: gamenet.PT_ASKINFO,
		},
		Version: doom.VERSION,
		Time:    uint32(time.Now().Unix()),
	}, server, ctx)
	if err != nil {
		// The server has stopped responding
		return true, nil
	}

	if si.NumberOfPlayer == 0 {
		// No players
		return true, nil
	}

	return false, nil
}

func (gi *GameInstance) Stop() error {
	timeout := 3 * time.Second
	return gi.client.ContainerStop(context.Background(), gi.id, &timeout)
}
