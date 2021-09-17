package main

import (
	"github.com/karashiiro/kartlobby/pkg/colortext"
	"github.com/karashiiro/kartlobby/pkg/gateway"
)

func main() {
	gs := gateway.NewServer(&gateway.GatewayOptions{
		Port:       5079,
		MaxClients: 15,
		Motd: colortext.
			New().
			AppendTextColored("kartlobby", colortext.Cyan).
			Build(),
	})
	defer gs.Close()
	gs.Run()
}
