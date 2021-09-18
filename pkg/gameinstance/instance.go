package gameinstance

import (
	"context"
	"fmt"
	"log"
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
	// WaitForMessage waits for a message with the provided opcode from the specified address.
	// This function should always be called with a timeout context in order to avoid hanging.
	WaitForMessage(message gamenet.Opcode, addr string, result chan []byte, err chan error, ctx context.Context)
}

type GameInstance struct {
	client *client.Client
	id     string
	port   int
	conn   network.Connection
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

	log.Println(port)

	// Create the container
	containerPort := nat.Port("5029/udp")
	resp, err := client.ContainerCreate(ctx, &container.Config{
		Image: GAMEIMAGE,
	}, &container.HostConfig{
		PortBindings: nat.PortMap{
			// Bind 5029/udp in the container to our free port on the host
			containerPort: []nat.PortBinding{{
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

	inst := &GameInstance{
		client: client,
		id:     resp.ID,
		port:   port,
		conn: network.NewUDPConnection(server, &net.UDPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: port,
		}),
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

	addr := gi.conn.Addr().String()

	// Launch goroutines waiting for the responses
	log.Println("Launching goroutines...")
	go server.WaitForMessage(gamenet.PT_SERVERINFO, addr, serverInfoChan, siErrChan, ctx)
	go server.WaitForMessage(gamenet.PT_PLAYERINFO, addr, playerInfoChan, piErrChan, ctx)

	// Forward the PT_ASKINFO request from the caller
	err := gamenet.SendPacket(gi.conn, askInfo)
	if err != nil {
		return nil, nil, err
	}

	// Wait for a result
	log.Println("Waiting for results...")
	serverInfoBytes := <-serverInfoChan
	playerInfoBytes := <-playerInfoChan

	// Error checking; context cancellations
	log.Println("Checking errors...")
	siErr := <-siErrChan
	if siErr != nil {
		return nil, nil, siErr
	}

	piErr := <-piErrChan
	if piErr != nil {
		return nil, nil, piErr
	}

	// Unmarshalling and returning results
	log.Println("Sending replies...")
	serverInfo := gamenet.ServerInfoPak{}
	playerInfo := gamenet.PlayerInfoPak{}

	gamenet.ReadPacket(serverInfoBytes, &serverInfo)
	gamenet.ReadPacket(playerInfoBytes, &playerInfo)

	return &serverInfo, &playerInfo, nil
}
