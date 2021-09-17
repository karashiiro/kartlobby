package gamenet

type Opcode = uint8

const (
	PT_NOTHING   Opcode = iota // To send a nop through the network. ^_~
	PT_SERVERCFG               // Server config used in start game
	// (must stay 1 for backwards compatibility).
	// This is a positive response to a CLIENTJOIN request.
	PT_CLIENTCMD     // Ticcmd of the client.
	PT_CLIENTMIS     // Same as above with but saying resend from.
	PT_CLIENT2CMD    // 2 cmds in the packet for splitscreen.
	PT_CLIENT2MIS    // Same as above with but saying resend from
	PT_NODEKEEPALIVE // Same but without ticcmd and consistancy
	PT_NODEKEEPALIVEMIS
	PT_SERVERTICS   // All cmds for the tic.
	PT_SERVERREFUSE // Server refuses joiner (reason inside).
	PT_SERVERSHUTDOWN
	PT_CLIENTQUIT // Client closes the connection.

	PT_ASKINFO      // Anyone can ask info of the server.
	PT_SERVERINFO   // Send game & server info (gamespy).
	PT_PLAYERINFO   // Send information for players in game (gamespy).
	PT_REQUESTFILE  // Client requests a file transfer
	PT_ASKINFOVIAMS // Packet from the MS requesting info be sent to new client.
	// If this ID changes, update masterserver definition.
	PT_RESYNCHEND // Player is now resynched and is being requested to remake the gametic
	PT_RESYNCHGET // Player got resynch packet

	// Add non-PT_CANFAIL packet types here to avoid breaking MS compatibility.

	// Kart-specific packets
	PT_CLIENT3CMD // 3P
	PT_CLIENT3MIS
	PT_CLIENT4CMD // 4P
	PT_CLIENT4MIS
	PT_BASICKEEPALIVE // Keep the network alive during wipes, as tics aren't advanced and NetUpdate isn't called

	PT_CANFAIL // This is kind of a priority. Anything bigger than CANFAIL
	// allows HSendPacket(*, true, *, *) to return false.
	// In addition, this packet can't occupy all the available slots.

	PT_FILEFRAGMENT = PT_CANFAIL // A part of a file.

	PT_TEXTCMD     = iota - 1 // Extra text commands from the client.
	PT_TEXTCMD2               // Splitscreen text commands.
	PT_TEXTCMD3               // 3P
	PT_TEXTCMD4               // 4P
	PT_CLIENTJOIN             // Client wants to join; used in start game.
	PT_NODETIMEOUT            // Packet sent to self if the connection times out.
	PT_RESYNCHING             // Packet sent to resync players.
	// Blocks game advance until synched.

	PT_TELLFILESNEEDED // Client, to server: "what other files do I need starting from this number?"
	PT_MOREFILESNEEDED // Server, to client: "you need these (+ more on top of those)"

	PT_PING // Packet sent to tell clients the other client's latency to server.
	NUMPACKETTYPE
)
