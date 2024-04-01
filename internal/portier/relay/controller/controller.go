package controller

import (
	"fmt"
	"log"
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
	AddConnection(messages.ConnectionID, adapter.ConnectionAdapter) error

	// Returns the event channel
	EventChannel() chan adapter.AdapterEvent
}

type controller struct {
	// uplink
	uplink uplink.Uplink

	// map of connection id to connection adapter
	connections map[messages.ConnectionID]adapter.ConnectionAdapter

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
		connections:        make(map[messages.ConnectionID]adapter.ConnectionAdapter),
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
				err := connectionAdapter.Stop()
				if err != nil {
					log.Printf("error stopping connection adapter: %s\n", err)
				}
				c.router.RemoveConnection(event.ConnectionId)
				delete(c.connections, event.ConnectionId)
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

func (c *controller) AddConnection(connectionID messages.ConnectionID, connectionAdapter adapter.ConnectionAdapter) error {
	if _, ok := c.connections[connectionID]; ok {
		return fmt.Errorf("connection with id %s already exists", connectionID)
	}

	c.connections[connectionID] = connectionAdapter

	c.router.AddConnection(connectionID, connectionAdapter)

	return nil
}

// CreateInboundConnection creates an inbound connection.
func (c *controller) CreateInboundConnection(header messages.MessageHeader, bridgeOptions messages.BridgeOptions, pcKey string) {
	// create a new inbound connection adapter
	connectionAdapter := adapter.NewInboundConnectionAdapter(adapter.ConnectionAdapterOptions{
		ConnectionId:          header.CID,
		LocalDeviceId:         header.To,
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

func (c *controller) EventChannel() chan adapter.AdapterEvent {
	return c.eventChannel
}
