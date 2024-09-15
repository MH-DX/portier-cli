package router

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/marinator86/portier-cli/internal/portier/ptls"
	"github.com/marinator86/portier-cli/internal/portier/relay/adapter"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
)

type Router interface {
	// Start starts the router
	Start() error

	// HandleMessage handles a message, i.e. creates a new service if necessary and routes the message to the service,
	// or routes the message to the existing service, or shuts down the service if the message is a shutdown message.
	// Returns an error if the message could not be routed.
	HandleMessage(msg messages.Message)

	// AddConnection adds a connection to the router
	AddConnection(messages.ConnectionID, adapter.ConnectionAdapter)

	// RemoveConnection removes a connection from the router
	RemoveConnection(messages.ConnectionID)

	EventChannel() chan adapter.AdapterEvent
}

type router struct {
	// services is the map of service connection id to service
	connections map[messages.ConnectionID]adapter.ConnectionAdapter

	// encoderDecoder is the encoder/decoder
	encoderDecoder encoder.EncoderDecoder

	// uplink is the uplink
	uplink uplink.Uplink

	// channel to receive messages from the uplink
	messages <-chan messages.Message

	// event channel
	events chan adapter.AdapterEvent

	// mutex to protect the services map
	mutex sync.Mutex

	// ptls is the ptls instance
	ptls ptls.PTLS
}

// NewRouter creates a new router.
func NewRouter(uplink uplink.Uplink, msg <-chan messages.Message, events chan adapter.AdapterEvent, ptls ptls.PTLS) Router {
	return &router{
		connections:    make(map[messages.ConnectionID]adapter.ConnectionAdapter),
		encoderDecoder: encoder.NewEncoderDecoder(),
		uplink:         uplink,
		messages:       msg,
		events:         events,
		mutex:          sync.Mutex{},
		ptls:           ptls,
	}
}

// Start starts the router.
func (r *router) Start() error {
	// start goroutine to handle messages
	go func() {
		for msg := range r.messages {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("recovered from panic: %v\n", r)
					}
				}()
				r.HandleMessage(msg)
			}()
		}
	}()

	go func() {
		// iterate over event channel
		for event := range r.events {
			log.Printf("event: %v\n", event)
			// get connection adapter
			connectionAdapter, ok := r.connections[event.ConnectionId]
			if !ok {
				// connection not found
				continue
			}
			// if event is close event, close connection
			if event.Type == adapter.Closed || event.Type == adapter.Error {
				err := connectionAdapter.Close()
				if err != nil {
					log.Printf("error stopping connection adapter: %s\n", err)
				}
				r.RemoveConnection(event.ConnectionId)
				continue
			}
		}
	}()

	return nil
}

// HandleMessage handles a message, i.e. creates a new service if necessary and routes the message to the service,
// or routes the message to the existing service, or shuts down the service if the message is a shutdown message.
// Returns an error if the message could not be routed.
func (r *router) HandleMessage(msg messages.Message) {
	r.mutex.Lock()

	// if connection exists, route to connection
	if connection, ok := r.connections[msg.Header.CID]; ok {
		r.mutex.Unlock()
		connection.Send(msg)
		return
	}

	defer r.mutex.Unlock()

	// if connection does not exist, and message is a ConnectionOpenMessage, create a new connection using the connection provider
	if msg.Header.Type == messages.CO {
		// decode the message into a ConnectionOpenMessage
		log.Printf("received connection open message for connection %s\n", msg.Header.CID)
		connectionOpenMessage, err := r.encoderDecoder.DecodeConnectionOpenMessage(msg.Message)
		if err != nil {
			log.Printf("error decoding connection open message: %v\n", err)
			log.Printf("message: %v\n", msg)
			return
		}
		r.CreateInboundConnection(msg.Header, connectionOpenMessage.BridgeOptions)
		return
	}

	if msg.Header.Type != messages.NF {
		log.Printf("received message for unknown connection %s, type %s\n",
			msg.Header.CID, msg.Header.Type)
		// send a not found message
		notFoundMessage := messages.Message{
			Header: messages.MessageHeader{
				From: msg.Header.To,
				To:   msg.Header.From,
				Type: messages.NF,
				CID:  msg.Header.CID,
			},
			Message: []byte{},
		}
		r.uplink.Send(notFoundMessage)
	}
}

// AddConnection adds an outbound connection to the router.
func (r *router) AddConnection(connectionId messages.ConnectionID, connection adapter.ConnectionAdapter) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.connections[connectionId] = connection
	log.Printf("added connection %s\n", connectionId)
}

// RemoveConnection removes a connection from the router.
func (r *router) RemoveConnection(connectionId messages.ConnectionID) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.connections, connectionId)
	log.Printf("removed connection %s\n", connectionId)
}

// CreateInboundConnection creates an inbound connection.
func (r *router) CreateInboundConnection(header messages.MessageHeader, bridgeOptions messages.BridgeOptions) {
	// create a new inbound connection adapter
	connectionAdapter := adapter.NewInboundConnectionAdapter(adapter.ConnectionAdapterOptions{
		ConnectionId:          header.CID,
		LocalDeviceId:         header.To,
		PeerDeviceId:          header.From,
		BridgeOptions:         bridgeOptions,
		ResponseInterval:      1000 * time.Millisecond,
		ConnectionReadTimeout: 1000 * time.Millisecond,
		ReadBufferSize:        1024,
		// TODO create a default config
	}, r.uplink, r.events, r.ptls)

	// start the connection adapter
	err := connectionAdapter.Start()
	if err != nil {
		fmt.Printf("error starting connection adapter: %s\n", err)
		return
	}
	log.Printf("started connection adapter for connection %s\n", header.CID)

	r.connections[header.CID] = connectionAdapter
	log.Printf("added connection %s\n", header.CID)
}

// EventChannel returns the event channel.
func (r *router) EventChannel() chan adapter.AdapterEvent {
	return r.events
}
