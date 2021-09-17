package gateway

import (
	"log"
	"net"

	"github.com/karashiiro/kartlobby/pkg/gamenet"
	"github.com/karashiiro/kartlobby/pkg/motd"
	"github.com/karashiiro/kartlobby/pkg/network"
)

type GatewayServer struct {
	port       int
	maxClients int
	server     *net.UDPConn
	clients    map[string]*clientInfo
	broadcast  *network.BroadcastConnection
	motd       *motd.MotdProvider
}

type GatewayOptions struct {
	Port       int
	MaxClients int
	Motd       string
}

func NewServer(opts *GatewayOptions) *GatewayServer {
	gs := GatewayServer{
		port:       opts.Port,
		maxClients: opts.MaxClients,
		clients:    make(map[string]*clientInfo),
		broadcast:  network.NewBroadcastConnection(opts.MaxClients),
		motd:       motd.New(opts.Motd),
	}
	return &gs
}

func (gs *GatewayServer) Close() {
	if gs.server == nil {
		return
	}

	shutdown := gamenet.PacketHeader{
		PacketType: gamenet.PT_SERVERSHUTDOWN,
	}
	gamenet.SendPacket(gs.broadcast, &shutdown)
	gs.server.Close()
}

func (gs *GatewayServer) Run() {
	server, err := net.ListenUDP("udp", &net.UDPAddr{Port: gs.port})
	if err != nil {
		log.Fatalln(err)
	}
	gs.server = server

	for {
		// PT_SERVERINFO should be the largest packet at 1024 bytes.
		// d_clisrv.h notes 64kB packets under doomdata_t, but those
		// are probably junk numbers.
		data := make([]byte, 1024)
		_, _, err := gs.server.ReadFrom(data)
		if err != nil {
			log.Fatalln(err)
		}
	}
}
