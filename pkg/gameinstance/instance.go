package gameinstance

import (
	"context"
	"fmt"
	"net"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/karashiiro/kartlobby/pkg/gamenet"
	"github.com/karashiiro/kartlobby/pkg/network"
)

const GAMEIMAGE = "brianallred/srb2kart"

type UDPServer interface {
	// WaitForMessage waits for a message with the provided opcode from the specified internal port.
	// This function should always be called with a timeout context in order to avoid hanging.
	WaitForInstanceMessage(key *UDPCallbackKey, result chan []byte, err chan error, ctx context.Context)
}

type UDPCallbackKey struct {
	Port    int
	Message gamenet.Opcode
}

type GameInstance struct {
	Conn network.Connection

	client *client.Client
	id     string
	port   int
}

func newInstance(server *net.UDPConn) (*GameInstance, error) {
	ctx := context.Background()
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	// Get a free port
	port, err := network.GetFreePort()
	if err != nil {
		return nil, err
	}

	// Create the container
	resp, err := client.ContainerCreate(ctx, &container.Config{
		Image: GAMEIMAGE,
	}, &container.HostConfig{
		PortBindings: nat.PortMap{
			// Bind 5029/udp in the container to our free port on the host
			nat.Port("5029/udp"): []nat.PortBinding{{
				HostPort: fmt.Sprint(port),
			}},
		},
	}, nil, nil, "")
	if err != nil {
		return nil, err
	}

	// Start the container
	if err := client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	// Get the host IP
	ip, err := network.GetLocalIP()
	if err != nil {
		return nil, err
	}

	inst := &GameInstance{
		Conn: network.NewUDPConnection(server, &net.UDPAddr{
			IP:   *ip,
			Port: port,
		}),

		client: client,
		id:     resp.ID,
		port:   port,
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
		Port:    gi.port,
		Message: gamenet.PT_SERVERINFO,
	}, serverInfoChan, siErrChan, ctx)
	go server.WaitForInstanceMessage(&UDPCallbackKey{
		Port:    gi.port,
		Message: gamenet.PT_PLAYERINFO,
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
