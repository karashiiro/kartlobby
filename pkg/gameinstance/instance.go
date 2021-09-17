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

type UDPServer interface {
	// WaitForMessage waits for a message with the provided opcode from the specified address.
	// This function should always be called with a timeout context in order to avoid hanging.
	WaitForMessage(message gamenet.Opcode, addr string, result chan []byte, err chan error, ctx context.Context)
}

type GameInstance struct {
	client *client.Client
	id     string
	conn   network.Connection
	hijack *types.HijackedResponse
}

func newInstance(server *net.UDPConn) (*GameInstance, error) {
	ctx := context.Background()
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	port, err := network.GetFreePort()
	if err != nil {
		return nil, err
	}

	exposedPort := nat.Port(fmt.Sprint(port) + ":5029/udp")

	resp, err := client.ContainerCreate(ctx, &container.Config{
		ExposedPorts: nat.PortSet{
			exposedPort: struct{}{},
		},
		Image: "brianallred/srb2kart",
	}, nil, nil, nil, "")
	if err != nil {
		return nil, err
	}

	if err := client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	hijack, err := client.ContainerAttach(ctx, resp.ID, types.ContainerAttachOptions{})
	if err != nil {
		return nil, err
	}

	inst := &GameInstance{
		client: client,
		id:     resp.ID,
		conn:   network.NewUDPConnection(server, hijack.Conn.RemoteAddr()),
		hijack: &hijack,
	}

	return inst, nil
}

func (gi *GameInstance) Close() {
	gi.hijack.Close()
}

func (gi *GameInstance) GetID() string {
	return gi.id
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
	go server.WaitForMessage(gamenet.PT_SERVERINFO, addr, serverInfoChan, siErrChan, ctx)
	go server.WaitForMessage(gamenet.PT_PLAYERINFO, addr, playerInfoChan, piErrChan, ctx)

	// Forward the PT_ASKINFO request from the caller
	err := gamenet.SendPacket(gi.conn, askInfo)
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

	gamenet.ReadPacket(serverInfoBytes, &serverInfo)
	gamenet.ReadPacket(playerInfoBytes, &playerInfo)

	return &serverInfo, &playerInfo, nil
}
