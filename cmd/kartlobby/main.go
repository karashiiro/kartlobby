package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/jinzhu/configor"
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
	// Flag parsing
	configPath := flag.String("config", "config.yml", "The configuration file path")
	flag.Parse()

	// Load configuration
	var config Configuration
	err := configor.Load(&config, *configPath)
	if err != nil {
		log.Fatalln(err)
	}

	// Create gateway server
	gs, err := gateway.NewServer(&gateway.GatewayOptions{
		Port:         config.GatewayPort,
		MaxInstances: config.MaxRooms,
		Motd: colortext.
			New().
			AppendTextColored("kartlobby", colortext.Cyan).
			Build(),
		DockerImage: config.DockerImage,
	})
	if err != nil {
		log.Fatalln(err)
	}
	defer gs.Close()

	// Create API
	r := rest.NewServer(&rest.RESTServerOptions{
		Port: config.APIPort,
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
