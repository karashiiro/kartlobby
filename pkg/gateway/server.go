package gateway

import (
	"context"
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
	callbacks  map[string]func(network.Connection, []byte)
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
		callbacks:  make(map[string]func(network.Connection, []byte)),
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
		_, addr, err := gs.server.ReadFrom(data)
		if err != nil {
			log.Fatalln(err)
		}

		conn := network.NewUDPConnection(gs.server, addr)

		// Check if we have any callbacks registered and run them if so.
		if cb, ok := gs.callbacks[addr.String()]; ok {
			go cb(conn, data)
		}

		go gs.handlePacket(conn, data)
	}
}

// WaitForMessage waits for a message with the provided opcode from the specified address.
// This function should always be called with a timeout context in order to avoid hanging.
func (gs *GatewayServer) WaitForMessage(message gamenet.Opcode, addr string, result chan []byte, err chan error, ctx context.Context) {
	got := make(chan bool, 1)

	// Register a callback for the message we want
	gs.callbacks[addr] = func(conn network.Connection, data []byte) {
		header := gamenet.PacketHeader{}
		gamenet.ReadPacket(data, &header)

		log.Printf("Got packet from %s with type %d", conn.Addr().String(), header.PacketType)

		if header.PacketType == message {
			// Unregister this function once we get the message
			delete(gs.callbacks, addr)

			result <- data
			got <- true
		}

		if _, ok := <-ctx.Done(); ok {
			err <- ctx.Err()
			got <- true
		}
	}

	<-got
}

func (gs *GatewayServer) handlePacket(conn network.Connection, data []byte) {
	header := gamenet.PacketHeader{}
	gamenet.ReadPacket(data, &header)

	log.Printf("Got packet from %s with type %d", conn.Addr().String(), header.PacketType)

	switch header.PacketType {
	case gamenet.PT_ASKINFO:
		askInfo := gamenet.AskInfoPak{}
		gamenet.ReadPacket(data, &askInfo)
	default:
		log.Println("Got unknown packet, forwarding")
	}
}
