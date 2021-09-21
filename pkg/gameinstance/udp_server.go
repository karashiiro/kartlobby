package gameinstance

import (
	"context"

	"github.com/karashiiro/kartlobby/pkg/gamenet"
)

type UDPServer interface {
	// WaitForMessage waits for a message with the provided opcode from the specified internal port.
	// This function should always be called with a timeout context in order to avoid hanging.
	WaitForInstanceMessage(key *UDPCallbackKey, result chan []byte, err chan error, ctx context.Context)
}

type UDPCallbackKey struct {
	Context  string
	GamePort int
	Message  gamenet.Opcode
}
