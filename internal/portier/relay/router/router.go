package router

import (
	"fmt"

	"github.com/marinator86/portier-cli/internal/portier/relay/adapter"
	"github.com/marinator86/portier-cli/internal/portier/relay/connector"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
)

type Router interface {
	// HandleMessage handles a message, i.e. creates a new service if necessary and routes the message to the service,
	// or routes the message to the existing service, or shuts down the service if the message is a shutdown message.
	// Returns an error if the message could not be routed.
	HandleMessage(msg messages.Message) error

	// AddConnection adds a connection to the router
	AddConnection(messages.ConnectionId, adapter.ConnectionAdapter)

	// RemoveConnection removes a connection from the router
	RemoveConnection(messages.ConnectionId)
}

type router struct {
	// services is the map of service connection id to service
	connections map[messages.ConnectionId]adapter.ConnectionAdapter

	// encoderDecoder is the encoder/decoder
	encoderDecoder encoder.EncoderDecoder

	// connector is the connector
	connector connector.Connector
}

// NewRouter creates a new router
func NewRouter() Router {
	return &router{
		connections:    make(map[messages.ConnectionId]adapter.ConnectionAdapter),
		encoderDecoder: encoder.NewEncoderDecoder(),
	}
}

// HandleMessage handles a message, i.e. creates a new service if necessary and routes the message to the service,
// or routes the message to the existing service, or shuts down the service if the message is a shutdown message.
// Returns an error if the message could not be routed.
func (r *router) HandleMessage(msg messages.Message) error {
	// if connection exists, route to connection
	if connection, ok := r.connections[msg.Header.CID]; ok {
		return connection.Send(msg)
	}

	// if connection does not exist, and message is a ConnectionOpenMessage, create a new connection using the connection provider
	if msg.Header.Type == messages.CO {
		// decode the message into a ConnectionOpenMessage
		connectionOpenMessage, err := r.encoderDecoder.DecodeConnectionOpenMessage(msg.Message)
		if err != nil {
			return err
		}

		return r.connector.CreateInboundConnection(msg.Header, connectionOpenMessage.BridgeOptions, connectionOpenMessage.PCKey)
	}

	// if connection does not exist, and message is not a ConnectionOpenMessage, return an error
	return fmt.Errorf("connection does not exist for connection id %s", msg.Header.CID)
}

// AddConnection adds an outbound connection to the router
func (r *router) AddConnection(connectionId messages.ConnectionId, connection adapter.ConnectionAdapter) {
	r.connections[connectionId] = connection
}

// RemoveConnection removes a connection from the router
func (r *router) RemoveConnection(connectionId messages.ConnectionId) {
	delete(r.connections, connectionId)
}
