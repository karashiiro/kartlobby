package main

import (
	"log"

	"github.com/karashiiro/kartlobby/pkg/colortext"
	"github.com/karashiiro/kartlobby/pkg/gateway"
	"github.com/karashiiro/kartlobby/pkg/rest"
)

func runApplicationLoop(fn func() error, errChan chan error) {
	err := fn()
	if err != nil {
		errChan <- err
	}
}

func main() {
	gs := gateway.NewServer(&gateway.GatewayOptions{
		Port:       5029,
		MaxClients: 15,
		Motd: colortext.
			New().
			AppendTextColored("kartlobby", colortext.Cyan).
			Build(),
	})
	defer gs.Close()

	r := rest.NewServer(&rest.RESTServerOptions{
		Port: 5030,
	})

	// TODO: make this not a single point of failure
	// that kicks everyone when a panic occurs
	errChan := make(chan error, 1)
	go runApplicationLoop(gs.Run, errChan)
	go runApplicationLoop(r.Run, errChan)

	err := <-errChan
	if err != nil {
		log.Fatalln(err)
	}

}
