package gameinstance

import (
	"github.com/karashiiro/kartlobby/pkg/gamenet"
	"github.com/karashiiro/kartlobby/pkg/network"
)

type GameInstance struct {
	conn network.Connection
}

func (gi *GameInstance) AskInfo() (*gamenet.ServerInfoPak, *gamenet.PlayerInfoPak, error) {
	return nil, nil, nil
}
