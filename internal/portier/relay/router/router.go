package router

import (
	"fmt"

	"github.com/marinator86/portier-cli/internal/portier/relay/adapter"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
)

type ConnectionOpenEvent struct {
	// message header
	Header messages.MessageHeader

	// bridge options
	BridgeOptions messages.BridgeOptions

	// pc key
	PCKey string
}

type Router interface {
	// Start starts the router
	Start() error

	// HandleMessage handles a message, i.e. creates a new service if necessary and routes the message to the service,
	// or routes the message to the existing service, or shuts down the service if the message is a shutdown message.
	// Returns an error if the message could not be routed.
	HandleMessage(msg messages.Message)

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

	// uplink is the uplink
	uplink uplink.Uplink

	// channel to receive messages from the uplink
	messages <-chan messages.Message

	// channel to push events to the controller
	events chan<- ConnectionOpenEvent
}

// NewRouter creates a new router
func NewRouter(uplink uplink.Uplink, msg <-chan messages.Message, events chan<- ConnectionOpenEvent) Router {
	return &router{
		connections:    make(map[messages.ConnectionId]adapter.ConnectionAdapter),
		encoderDecoder: encoder.NewEncoderDecoder(),
		uplink:         uplink,
		messages:       msg,
		events:         events,
	}
}

// Start starts the router
func (r *router) Start() error {
	// start goroutine to handle messages
	go func() {
		for msg := range r.messages {
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("recovered from panic: %v\n", r)
					}
				}()
				r.HandleMessage(msg)
			}()
		}
	}()
	return nil
}

// HandleMessage handles a message, i.e. creates a new service if necessary and routes the message to the service,
// or routes the message to the existing service, or shuts down the service if the message is a shutdown message.
// Returns an error if the message could not be routed.
func (r *router) HandleMessage(msg messages.Message) {
	// if connection exists, route to connection
	if connection, ok := r.connections[msg.Header.CID]; ok {
		connection.Send(msg)
		return
	}

	// if connection does not exist, and message is a ConnectionOpenMessage, create a new connection using the connection provider
	if msg.Header.Type == messages.CO {
		// decode the message into a ConnectionOpenMessage
		connectionOpenMessage, err := r.encoderDecoder.DecodeConnectionOpenMessage(msg.Message)
		if err != nil {
			fmt.Printf("error decoding connection open message: %v\n", err)
			fmt.Printf("message: %v\n", msg)
			return
		}
		r.events <- ConnectionOpenEvent{
			Header:        msg.Header,
			BridgeOptions: connectionOpenMessage.BridgeOptions,
			PCKey:         connectionOpenMessage.PCKey,
		}
		return
	}

	// send connection not found message in any case
	connectionNotFoundMessage := messages.Message{
		Header: messages.MessageHeader{
			From: msg.Header.To,
			To:   msg.Header.From,
			Type: messages.NF,
			CID:  msg.Header.CID,
		},
		Message: []byte(""),
	}
	err := r.uplink.Send(connectionNotFoundMessage)
	if err != nil {
		fmt.Printf("error sending connection not found message: %v\n", err)
	}
}

// AddConnection adds an outbound connection to the router
func (r *router) AddConnection(connectionId messages.ConnectionId, connection adapter.ConnectionAdapter) {
	r.connections[connectionId] = connection
}

// RemoveConnection removes a connection from the router
func (r *router) RemoveConnection(connectionId messages.ConnectionId) {
	delete(r.connections, connectionId)
}
