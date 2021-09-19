package main

import (
	"context"
	"log"
	"time"

	"github.com/karashiiro/kartlobby/pkg/colortext"
	"github.com/karashiiro/kartlobby/pkg/doom"
	"github.com/karashiiro/kartlobby/pkg/gamenet"
	"github.com/karashiiro/kartlobby/pkg/gateway"
	"github.com/karashiiro/kartlobby/pkg/rest"
)

type message struct {
	Msg string `json:"msg"`
}

type askInfoResponse struct {
	ServerInfo *gamenet.ServerInfoPak
	PlayerInfo *gamenet.PlayerInfoPak
}

func runApplicationLoop(fn func() error, errChan chan error) {
	err := fn()
	if err != nil {
		errChan <- err
	}
}

func main() {
	gs, err := gateway.NewServer(&gateway.GatewayOptions{
		Port:         5029,
		MaxInstances: 1,
		Motd: colortext.
			New().
			AppendTextColored("kartlobby", colortext.Cyan).
			Build(),
	})
	if err != nil {
		log.Fatalln(err)
	}
	defer gs.Close()

	r := rest.NewServer(&rest.RESTServerOptions{
		Port: 5030,
	})

	r.Get("/new", func() (interface{}, error) {
		_, err := gs.Instances.CreateInstance(gs.Server)
		if err != nil {
			return nil, err
		}

		return &message{Msg: "Success"}, nil
	})

	r.Get("/askinfo", func() (interface{}, error) {
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		si, pi, err := gs.Instances.AskInfo(&gamenet.AskInfoPak{
			PacketHeader: gamenet.PacketHeader{
				PacketType: gamenet.PT_ASKINFO,
			},
			Version: doom.VERSION,
			Time:    uint32(time.Now().Unix()),
		}, gs, ctx)
		if err != nil {
			return nil, err
		}

		return &askInfoResponse{
			ServerInfo: si,
			PlayerInfo: pi,
		}, nil
	})

	// TODO: make this not a single point of failure
	// that kicks everyone when a panic occurs
	errChan := make(chan error, 1)
	go runApplicationLoop(gs.Run, errChan)
	go runApplicationLoop(r.Run, errChan)

	err = <-errChan
	if err != nil {
		log.Fatalln(err)
	}

}
