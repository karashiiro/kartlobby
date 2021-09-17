package gameinstance

import (
	"context"

	"github.com/karashiiro/kartlobby/pkg/gamenet"
	"github.com/karashiiro/kartlobby/pkg/network"
)

type UDPServer interface {
	// WaitForMessage waits for a message with the provided opcode from the specified address.
	// This function should always be called with a timeout context in order to avoid hanging.
	WaitForMessage(message gamenet.Opcode, addr string, result chan []byte, err chan error, ctx context.Context)
}

type GameInstance struct {
	conn network.Connection
}

func (gi *GameInstance) AskInfo(server UDPServer, ctx context.Context) (*gamenet.ServerInfoPak, *gamenet.PlayerInfoPak, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	serverInfoChan := make(chan []byte, 1)
	playerInfoChan := make(chan []byte, 1)

	siErrChan := make(chan error, 1)
	piErrChan := make(chan error, 1)

	addr := gi.conn.Addr().String()
	go server.WaitForMessage(gamenet.PT_SERVERINFO, addr, serverInfoChan, siErrChan, ctx)
	go server.WaitForMessage(gamenet.PT_PLAYERINFO, addr, playerInfoChan, piErrChan, ctx)

	askInfo := gamenet.AskInfoPak{}
	err := gamenet.SendPacket(gi.conn, &askInfo)
	if err != nil {
		return nil, nil, err
	}

	serverInfoBytes := <-serverInfoChan
	playerInfoBytes := <-playerInfoChan

	siErr := <-siErrChan
	if siErr != nil {
		return nil, nil, siErr
	}

	piErr := <-piErrChan
	if piErr != nil {
		return nil, nil, piErr
	}

	serverInfo := gamenet.ServerInfoPak{}
	playerInfo := gamenet.PlayerInfoPak{}

	gamenet.ReadPacket(serverInfoBytes, &serverInfo)
	gamenet.ReadPacket(playerInfoBytes, &playerInfo)

	return &serverInfo, &playerInfo, nil
}