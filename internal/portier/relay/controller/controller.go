package controller

import (
	"fmt"
	"time"

	"github.com/marinator86/portier-cli/internal/portier/relay/adapter"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/router"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
)

type Controller interface {
	// Start starts the controller
	Start() error

	// AddConnection adds a connection to the controller
	AddConnection(messages.ConnectionId, adapter.ConnectionAdapter) error
}

type controller struct {
	// uplink
	uplink uplink.Uplink

	// map of connection id to connection adapter
	connections map[messages.ConnectionId]adapter.ConnectionAdapter

	// event channel
	eventChannel chan adapter.AdapterEvent

	// router event channel
	routerEventChannel chan router.ConnectionOpenEvent

	// router
	router router.Router
}

func NewController(uplink uplink.Uplink, eventChannel chan adapter.AdapterEvent, routerEventChannel chan router.ConnectionOpenEvent, router router.Router) Controller {
	return &controller{
		uplink:             uplink,
		connections:        make(map[messages.ConnectionId]adapter.ConnectionAdapter),
		eventChannel:       eventChannel,
		routerEventChannel: routerEventChannel,
		router:             router,
	}
}

func (c *controller) Start() error {
	go func() {
		// iterate over event channel
		for event := range c.eventChannel {
			// get connection adapter
			connectionAdapter, ok := c.connections[event.ConnectionId]
			if !ok {
				// connection not found
				continue
			}
			// if event is close event, close connection
			if event.Type == adapter.Closed || event.Type == adapter.Error {
				_ = connectionAdapter.Stop()
				continue
			}
		}
	}()

	go func() {
		// iterate over router event channel
		for event := range c.routerEventChannel {
			// create inbound connection
			c.CreateInboundConnection(event.Header, event.BridgeOptions, event.PCKey)
		}
	}()
	return nil
}

func (c *controller) AddConnection(connectionId messages.ConnectionId, connectionAdapter adapter.ConnectionAdapter) error {
	if _, ok := c.connections[connectionId]; ok {
		return fmt.Errorf("connection with id %s already exists", connectionId)
	}

	c.connections[connectionId] = connectionAdapter

	c.router.AddConnection(connectionId, connectionAdapter)

	return nil
}

// CreateInboundConnection creates an inbound connection.
func (c *controller) CreateInboundConnection(header messages.MessageHeader, bridgeOptions messages.BridgeOptions, pcKey string) {
	// create a new inbound connection adapter
	connectionAdapter := adapter.NewInboundConnectionAdapter(adapter.ConnectionAdapterOptions{
		ConnectionId:          header.CID,
		PeerDeviceId:          header.From,
		PeerDevicePublicKey:   pcKey,
		BridgeOptions:         bridgeOptions,
		ResponseInterval:      1000 * time.Millisecond,
		ConnectionReadTimeout: 1000 * time.Millisecond,
		ReadBufferSize:        1024,
		// TODO create a default config
	}, c.uplink, c.eventChannel)

	// start the connection adapter
	err := connectionAdapter.Start()
	if err != nil {
		fmt.Printf("error starting connection adapter: %s\n", err)
		return
	}
	_ = c.AddConnection(header.CID, connectionAdapter)
}
