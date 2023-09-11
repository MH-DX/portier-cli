package controller

import (
	"github.com/marinator86/portier-cli/internal/portier/relay/adapter"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
)

type Controller interface {
	// AddConnection adds a connection
	AddConnection(messages.ConnectionId, adapter.ConnectionAdapter) error

	// GetEventChannel returns the event channel
	GetEventChannel() chan<- adapter.AdapterEvent
}
