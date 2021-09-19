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
	broadcast    *network.BroadcastConnection
	motd         motd.Motd

	// Connection map of both clients and servers
	peers      map[string]*peerInfo
	peersMutex *sync.Mutex

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
		broadcast:    network.NewBroadcastConnection(opts.MaxClients),
		motd:         motd.New(opts.Motd),

		peers:      make(map[string]*peerInfo),
		peersMutex: &sync.Mutex{},

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
		n, addr, err := gs.Server.ReadFrom(data)
		if err != nil {
			log.Println(err)
			continue
		}

		// Only process UDP connections
		if udpAddr, udpOk := addr.(*net.UDPAddr); udpOk {
			conn := network.NewUDPConnection(gs.Server, udpAddr)

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

			gs.internalCallbacksMutex.Lock()
			if cb, cbOk := gs.internalCallbacks[cbKey]; cbOk {
				go cb(conn, &header, data)
			} else {
				// Handle the packet normally
				go gs.handlePacket(conn, addr.(*net.UDPAddr), &header, data, n)
			}
			gs.internalCallbacksMutex.Unlock()
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

func (gs *GatewayServer) handlePacket(conn network.Connection, addr *net.UDPAddr, header *gamenet.PacketHeader, data []byte, n int) {
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
		var peer *peerInfo
		if knownPeer, ok := gs.peers[conn.Addr().String()]; ok {
			peer = knownPeer
		} else {
			gs.peersMutex.Lock()
			defer gs.peersMutex.Unlock()

			// Create a new map entry to forward this player's packets to a server.
			//
			// We need to make sure that the server reads the sender IP field
			// as the player's, and not ours. Because of this, we need to create
			// the connections in the sender info with *new* UDP receivers created
			// with DialUDP.

			var localAddr *net.UDPAddr = addr // The address that the message is coming *from*
			var remoteAddr *net.UDPAddr       // The address that we want messages from localAddr to go *to*
			var remoteConn network.Connection
			var proxy *net.UDPConn

			// The message is from a player, get or create an open instance
			inst, err := gs.Instances.GetOrCreateOpenInstance(gs.Server, gs)
			if err != nil {
				log.Println(err)
				return
			}

			// Get a free port to proxy through
			proxyPort, err := network.GetFreePort()
			if err != nil {
				log.Println(err)
				return
			}

			// Start a new UDP server on our proxy port
			proxy, err = net.ListenUDP("udp", &net.UDPAddr{Port: proxyPort})
			if err != nil {
				log.Println(err)
				return
			}

			// Create the player connection, which forwards messages received from
			// the proxy to a player
			playerConn := network.NewUDPConnection(gs.Server, localAddr)

			// TODO: This is an untracked goroutine and therefore potentially error-prone
			go func() {
				var proxyData [2048]byte
				for {
					// Read packets from the game
					n, _, err := proxy.ReadFrom(proxyData[:])
					if err != nil {
						log.Println(err)
						continue
					}

					// Forward packets to the client
					err = playerConn.Send(proxyData[:n])
					if err != nil {
						log.Println(err)
					}
				}
			}()

			// Set the remote address to the open instance
			remoteAddr = inst.Conn.Addr().(*net.UDPAddr)

			// Set the remote connection to one that directs messages from the
			// proxy server to the game
			remoteConn = network.NewUDPConnection(proxy, remoteAddr)

			peer = &peerInfo{
				localAddr:  localAddr,
				remoteConn: remoteConn,
				proxy:      proxy,
			}

			gs.peers[conn.Addr().String()] = peer
		}

		// Forward packet to intended recipient
		err := peer.remoteConn.Send(data[:n])
		if err != nil {
			log.Println(err)
			return
		}
	}
}
