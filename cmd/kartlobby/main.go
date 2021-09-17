package main

import "github.com/karashiiro/kartlobby/pkg/gateway"

func main() {
	gs := gateway.NewServer(5079)
	defer gs.Close()
	gs.Run()
}
