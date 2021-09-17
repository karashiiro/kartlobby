package gamenet

import "github.com/karashiiro/kartlobby/pkg/doom"

const PACKETVERSION = 0

type PacketHeader struct {
	Checksum  uint32
	Ack       uint8 // If not zero the node asks for acknowledgement, the receiver must resend the ack
	AckReturn uint8 // The return of the ack number

	PacketType Opcode
	Reserved   uint8 // Padding
}

const MAXFILENEEDED = 915
const MAX_MIRROR_LENGTH = 256

type ServerInfoPak struct {
	PacketHeader

	X255           uint8
	PacketVersion  uint8
	Application    [doom.MAXAPPLICATION]byte
	Version        uint8
	Subversion     uint8
	NumberOfPlayer uint8
	MaxPlayer      uint8
	GameType       uint8
	ModifiedGame   uint8
	CheatsEnabled  uint8
	KartVars       uint8
	FileNeededNum  uint8
	Time           doom.Tic
	LevelTime      doom.Tic
	ServerName     [32]byte
	MapName        [8]byte
	MapTitle       [33]byte
	MapMD5         [16]byte
	ActNum         uint8
	IsZone         uint8
	HttpSource     [MAX_MIRROR_LENGTH]byte
	FileNeeded     [MAXFILENEEDED]byte
}

type PlayerInfoPak struct {
	PacketHeader

	Players [doom.MASTERSERVER_MAXPLAYERS]PlayerInfo
}

// PlayerInfo represents shorter player information for external use.
type PlayerInfo struct {
	Node            uint8
	Name            [doom.MAXPLAYERNAME + 1]byte
	Reserved        [4]uint8
	Team            uint8
	Skin            uint8
	Data            uint8 // Color is first four bits, hasflag, isit and issuper have one bit each, the last is unused.
	Score           uint32
	SecondsInServer uint16
}

type ServerRefusePak struct {
	PacketHeader

	Reason [255]byte
}

type AskInfoPak struct {
	PacketHeader

	Version uint8
	Time    doom.Tic // used for ping evaluation
}

type MSAskInfoPak struct {
	PacketHeader

	ClientAddr [22]byte
	Time       doom.Tic // used for ping evaluation
}
