package gateway

import (
	"context"
	"log"
	"net"
	"sync"
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
	broadcast    *network.BroadcastConnection
	motd         motd.Motd

	// Callbacks for internal servers, keyed on the container port
	internalCallbacks      map[gameinstance.UDPCallbackKey]func(network.Connection, *gamenet.PacketHeader, []byte)
	internalCallbacksMutex *sync.Mutex
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
		broadcast:    network.NewBroadcastConnection(opts.MaxClients),
		motd:         motd.New(opts.Motd),

		internalCallbacks:      make(map[gameinstance.UDPCallbackKey]func(network.Connection, *gamenet.PacketHeader, []byte)),
		internalCallbacksMutex: &sync.Mutex{},
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
		return err
	}
	gs.Server = server

	for {
		// PT_SERVERINFO should be the largest packet at 1024 bytes.
		// d_clisrv.h notes 64kB packets under doomdata_t, but those
		// are probably junk numbers.
		//
		// Regardless of that, this returns an error showing that the
		// buffer is too small when the server sends that packet, so
		// we're just going to allocate double that amount.
		data := make([]byte, 2048)
		_, addr, err := gs.Server.ReadFrom(data)
		if err != nil {
			log.Println(err)
			continue
		}

		// Only process UDP connections
		if udpAddr, udpOk := addr.(*net.UDPAddr); udpOk {
			conn := network.NewUDPConnection(gs.Server, addr)

			// Check if we have any container message callbacks registered and run them if so.
			header := gamenet.PacketHeader{}
			err := gamenet.ReadPacket(data, &header)
			if err != nil {
				log.Println(err)
				continue
			}

			cbKey := gameinstance.UDPCallbackKey{
				Port:    udpAddr.Port,
				Message: header.PacketType,
			}

			if cb, cbOk := gs.internalCallbacks[cbKey]; cbOk {
				go cb(conn, &header, data)
			} else {
				// Handle the packet normally
				go gs.handlePacket(conn, addr.(*net.UDPAddr), &header, data)
			}
		}
	}
}

// WaitForInstanceMessage waits for a message with the provided opcode from the specified internal port.
// This function should always be called with a timeout context in order to avoid hanging.
func (gs *GatewayServer) WaitForInstanceMessage(key *gameinstance.UDPCallbackKey, result chan []byte, err chan error, ctx context.Context) {
	got := make(chan bool, 1)

	// Cleanup function
	onGot := func() {
		// Unregister the callback function
		gs.internalCallbacksMutex.Lock()
		delete(gs.internalCallbacks, *key)
		gs.internalCallbacksMutex.Unlock()

		got <- true

		close(result)
		close(err)
	}

	// Register a callback for the message we want
	gs.internalCallbacksMutex.Lock()
	gs.internalCallbacks[*key] = func(conn network.Connection, header *gamenet.PacketHeader, data []byte) {
		log.Printf("Callback (%v): Got packet from %s with type %d", *key, conn.Addr().String(), header.PacketType)

		if header.PacketType == key.Message {
			result <- data
			onGot()
		}
	}
	gs.internalCallbacksMutex.Unlock()

	// Context timeout check
	go func() {
		<-ctx.Done()
		select {
		case <-err:
			return
		default:
			err <- ctx.Err()
			onGot()
		}
	}()

	<-got
}

func (gs *GatewayServer) handlePacket(conn network.Connection, addr *net.UDPAddr, header *gamenet.PacketHeader, data []byte) {
	log.Printf("Got packet from %s with type %d", conn.Addr().String(), header.PacketType)

	switch header.PacketType {
	case gamenet.PT_ASKINFO:
		askInfo := gamenet.AskInfoPak{}
		err := gamenet.ReadPacket(data, &askInfo)
		if err != nil {
			log.Println(err)
			return
		}

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
