package gateway

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/karashiiro/kartlobby/pkg/gameinstance"
	"github.com/karashiiro/kartlobby/pkg/gamenet"
	"github.com/karashiiro/kartlobby/pkg/motd"
	"github.com/karashiiro/kartlobby/pkg/network"
)

type GatewayServer struct {
	Instances *gameinstance.GameInstanceManager
	Server    *net.UDPConn

	port         int
	maxClients   int
	maxInstances int
	clients      map[string]*clientInfo
	callbacks    map[string]func(network.Connection, []byte)
	broadcast    *network.BroadcastConnection
	motd         motd.Motd
}

type GatewayOptions struct {
	Port         int
	MaxClients   int
	MaxInstances int
	Motd         string
}

func NewServer(opts *GatewayOptions) *GatewayServer {
	gs := GatewayServer{
		Instances: gameinstance.NewManager(opts.MaxInstances),

		port:         opts.Port,
		maxClients:   opts.MaxClients,
		maxInstances: opts.MaxInstances,
		clients:      make(map[string]*clientInfo),
		callbacks:    make(map[string]func(network.Connection, []byte)),
		broadcast:    network.NewBroadcastConnection(opts.MaxClients),
		motd:         motd.New(opts.Motd),
	}
	return &gs
}

// Close shuts down the server, sending a PT_SERVERSHUTDOWN to all
// connected clients.
func (gs *GatewayServer) Close() {
	if gs.Server == nil {
		return
	}

	shutdown := gamenet.PacketHeader{
		PacketType: gamenet.PT_SERVERSHUTDOWN,
	}
	gamenet.SendPacket(gs.broadcast, &shutdown)
	gs.Server.Close()
}

// Run initializes the internal UDP server and blocks, looping while
// handling UDP messages.
func (gs *GatewayServer) Run() error {
	server, err := net.ListenUDP("udp", &net.UDPAddr{Port: gs.port})
	if err != nil {
		log.Fatalln(err)
	}
	gs.Server = server

	for {
		// PT_SERVERINFO should be the largest packet at 1024 bytes.
		// d_clisrv.h notes 64kB packets under doomdata_t, but those
		// are probably junk numbers.
		data := make([]byte, 1024)
		_, addr, err := gs.Server.ReadFrom(data)
		if err != nil {
			log.Println(err)
			continue
		}

		conn := network.NewUDPConnection(gs.Server, addr)

		// Check if we have any callbacks registered and run them if so.
		if cb, ok := gs.callbacks[addr.String()]; ok {
			go cb(conn, data)
		} else {
			go gs.handlePacket(conn, addr.(*net.UDPAddr), data)
		}
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
			// Unregister this function if we didn't get a message within the context bounds
			delete(gs.callbacks, addr)

			err <- ctx.Err()
			got <- true
		}
	}

	<-got
}

func (gs *GatewayServer) handlePacket(conn network.Connection, addr *net.UDPAddr, data []byte) {
	header := gamenet.PacketHeader{}
	gamenet.ReadPacket(data, &header)

	log.Printf("Got packet from %s with type %d", conn.Addr().String(), header.PacketType)

	switch header.PacketType {
	case gamenet.PT_ASKINFO:
		askInfo := gamenet.AskInfoPak{}
		gamenet.ReadPacket(data, &askInfo)

		// Prepare to wait up to 3 seconds for a server response
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		// Forward the request
		serverInfo, playerInfo, err := gs.Instances.AskInfo(&askInfo, gs, ctx)
		if err != nil {
			log.Println(err)
			return
		}

		// Overwrite the server name with our motd
		motd := []byte(gs.motd.GetMotd())
		for i := 0; i < len(serverInfo.ServerName); i++ {
			if i >= len(motd) {
				serverInfo.ServerName[i] = 0
			} else {
				serverInfo.ServerName[i] = motd[i]
			}
		}

		// Send responses
		err = gamenet.SendPacket(conn, serverInfo)
		if err != nil {
			log.Println(err)
			return
		}

		err = gamenet.SendPacket(conn, playerInfo)
		if err != nil {
			log.Println(err)
			return
		}
	default:
		if client, ok := gs.clients[conn.Addr().String()]; ok {
			log.Println("Got unknown packet, forwarding...")
			err := gamenet.SendPacket(client.gameConn, data)
			if err != nil {
				log.Println(err)
				return
			}
		} else {
			log.Println("Got unknown packet, can't do anything")
			// For future reference: We need to make sure that the server reads the sender IP field
			// as the client's, and not ours. Because of this, we need to create the connections in
			// the client info with *new* UDP servers created with DialUDP.

			/*var gameAddr *net.UDPAddr

			fromClient, err := net.DialUDP("udp", gameAddr, addr)
			if err != nil {
				log.Println(err)
				return
			}

			gameConn := network.NewUDPConnection(fromClient, gameAddr)

			gs.clients[conn.Addr().String()] = &clientInfo{
				gameConn: gameConn,
			}*/
		}
	}
}
