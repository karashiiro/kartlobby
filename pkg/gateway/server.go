package gateway

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
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
	maxInstances int
	broadcast    *network.BroadcastConnection
	motd         motd.Motd

	// Connection map from clients to servers
	clients      map[string]*gameProxy
	clientsMutex *sync.Mutex

	// Callbacks for internal servers
	internalCallbacks      map[string]func(network.Connection, *gamenet.PacketHeader, []byte)
	internalCallbacksMutex *sync.Mutex

	// Clients waiting for an instance to be created
	instanceCreationWaitTable *waitTable

	// Clients waiting to join an instance
	instanceJoinWaitTable *waitTable
}

type GatewayOptions struct {
	Port         int
	MaxInstances int
	Motd         string
}

func NewServer(opts *GatewayOptions) (*GatewayServer, error) {
	broadcast, err := network.NewBroadcastConnection(16)
	if err != nil {
		return nil, err
	}

	gs := GatewayServer{
		Instances: gameinstance.NewManager(opts.MaxInstances),

		port:         opts.Port,
		maxInstances: opts.MaxInstances,
		broadcast:    broadcast,
		motd:         motd.New(opts.Motd),

		clients:      make(map[string]*gameProxy),
		clientsMutex: &sync.Mutex{},

		internalCallbacks:      make(map[string]func(network.Connection, *gamenet.PacketHeader, []byte)),
		internalCallbacksMutex: &sync.Mutex{},

		instanceCreationWaitTable: newWaitTable(),
		instanceJoinWaitTable:     newWaitTable(),
	}

	return &gs, nil
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
		// we're just going to allocate double that amount. The packet
		// actually comes out to 1160 bytes, for some reason.
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

			gs.internalCallbacksMutex.Lock()
			// Run a callback if we have any matching ones
			ranCb := false
			for key, cb := range gs.internalCallbacks {
				if strings.HasPrefix(key, fmt.Sprintf("%d-%d", udpAddr.Port, header.PacketType)) {
					go cb(conn, &header, data)
					ranCb = true
					break
				}
			}

			// Otherwise, handle the packet normally
			if !ranCb {
				go gs.handlePacket(conn, udpAddr, &header, data, n)
			}
			gs.internalCallbacksMutex.Unlock()
		}
	}
}

// WaitForInstanceMessage waits for a message with the provided opcode from the specified internal port.
// If the context specified in the callback key already has callbacks registered for this message, an
// error is returned. This function should always be called with a timeout context in order to avoid hanging.
func (gs *GatewayServer) WaitForInstanceMessage(key *gameinstance.UDPCallbackKey, result chan []byte, errChan chan error, ctx context.Context) {
	// Create the key
	keyStr := fmt.Sprintf("%d-%d-%s", key.GamePort, key.Message, key.Context)

	// Check if we have a callback registered already
	gs.internalCallbacksMutex.Lock()
	if _, ok := gs.internalCallbacks[keyStr]; ok {
		result <- nil
		errChan <- errors.New("callback already registered")

		close(result)
		close(errChan)

		gs.internalCallbacksMutex.Unlock()
		return
	}
	gs.internalCallbacksMutex.Unlock()

	got := make(chan bool, 1)

	// Cleanup function
	onGot := func(data []byte, err error) {
		gs.internalCallbacksMutex.Lock()
		defer gs.internalCallbacksMutex.Unlock()

		if _, ok := gs.internalCallbacks[keyStr]; !ok {
			// We already removed the callback, this was a race condition
			got <- true
			return
		}

		// Unregister the callback function
		delete(gs.internalCallbacks, keyStr)

		// Send the data back
		result <- data
		errChan <- err

		close(result)
		close(errChan)

		got <- true
	}

	// Register a callback for the message we want
	gs.internalCallbacksMutex.Lock()
	gs.internalCallbacks[keyStr] = func(conn network.Connection, header *gamenet.PacketHeader, data []byte) {
		if header.PacketType == key.Message {
			log.Printf("Callback (%s): Got packet from %s with type %d", keyStr, conn.Addr().String(), header.PacketType)
			onGot(data, nil)
		}
	}
	gs.internalCallbacksMutex.Unlock()

	// Context timeout check
	go func() {
		<-ctx.Done()
		select {
		case <-errChan:
			return
		default:
			onGot(nil, ctx.Err())
		}
	}()

	<-got
}

func (gs *GatewayServer) handlePacket(conn network.Connection, addr net.Addr, header *gamenet.PacketHeader, data []byte, n int) {
	switch header.PacketType {
	case gamenet.PT_ASKINFO:
		askInfo := gamenet.AskInfoPak{}
		err := gamenet.ReadPacket(data, &askInfo)
		if err != nil {
			log.Println(err)
			return
		}

		// Check if we're already waiting, take a lock otherwise
		defer gs.instanceCreationWaitTable.LockUnlock()()
		if gs.instanceCreationWaitTable.IsSet(addr.String()) {
			return
		}

		// This will defer the unset, which will be pushed onto the defer
		// stack and be called *before* the deferred unlock
		defer gs.instanceCreationWaitTable.SetUnset(addr.String())()

		// Prepare to wait up to 3 seconds for a server response
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		// Forward the request
		serverInfo, playerInfo, err := gs.Instances.AskInfo(&askInfo, gs, ctx)
		if err != nil {
			// Get/create an instance and retry
			gs.Instances.GetOrCreateOpenInstance(gs.Server, gs)

			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			serverInfo, playerInfo, err = gs.Instances.AskInfo(&askInfo, gs, ctx)
			if err != nil {
				log.Println(err)
				return
			}
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
		var proxy *gameProxy
		if knownProxy, ok := gs.clients[conn.Addr().String()]; ok {
			proxy = knownProxy
		} else {
			gs.clientsMutex.Lock()
			defer gs.clientsMutex.Unlock()

			// Create a new map entry to forward this player's packets to a server.
			//
			// We need to make sure that the server reads the sender IP field
			// as the player's, and not ours. Because of this, we need to create
			// the connections in the client info with a *new* UDP server with a
			// port distinguished for this purpose.

			var clientAddr net.Addr = addr // The sender's address, renamed here for clarity.

			// Check if we're already waiting to join, take a lock otherwise
			defer gs.instanceJoinWaitTable.LockUnlock()()
			if gs.instanceJoinWaitTable.IsSet(addr.String()) {
				return
			}

			// This will defer the unset, which will be pushed onto the defer
			// stack and be called *before* the deferred unlock
			defer gs.instanceJoinWaitTable.SetUnset(addr.String())()

			// Get or create an open instance
			inst, err := gs.Instances.GetOrCreateOpenInstance(gs.Server, gs)
			if err != nil {
				log.Println(err)
				return
			}

			// Create the player connection, which forwards messages received from
			// the proxy to a player
			playerConn := network.NewUDPConnection(gs.Server, clientAddr)

			// Set the connection so we can broadcast global messages to it
			gs.broadcast.Set(playerConn)

			// Start a new UDP server to proxy through
			proxy, err = newGameProxy(playerConn, inst)
			if err != nil {
				log.Println(err)
				return
			}

			// TODO: This is an untracked goroutine and therefore potentially error-prone
			go proxy.Run()

			// Store the proxy for future packets
			gs.clients[conn.Addr().String()] = proxy
		}

		// Forward packet to the game
		err := proxy.SendToGame(data[:n])
		if err != nil {
			log.Println(err)
			return
		}
	}
}
