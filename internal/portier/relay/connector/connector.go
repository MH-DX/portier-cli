package connector

import (
	"fmt"
	"time"

	"github.com/marinator86/portier-cli/internal/portier/relay/adapter"
	"github.com/marinator86/portier-cli/internal/portier/relay/controller"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
)

type Connector interface {
	// CreateConnection creates a new connection
	CreateInboundConnection(header messages.MessageHeader, options messages.BridgeOptions, pcKey string)
}

type connector struct {
	uplink uplink.Uplink

	encoderDecoder encoder.EncoderDecoder

	controller controller.Controller
}

// NewConnector creates a new connector
func NewConnector(uplink uplink.Uplink, ctrl controller.Controller) Connector {
	return &connector{
		uplink:         uplink,
		encoderDecoder: encoder.NewEncoderDecoder(),
		controller:     ctrl,
	}
}

// CreateInboundConnection creates an inbound connection
func (c *connector) CreateInboundConnection(header messages.MessageHeader, bridgeOptions messages.BridgeOptions, pcKey string) {
	// create a new inbound connection adapter
	connectionAdapter := adapter.NewInboundConnectionAdapter(adapter.ConnectionAdapterOptions{
		ConnectionId:        header.CID,
		PeerDeviceId:        header.From,
		PeerDevicePublicKey: pcKey,
		BridgeOptions:       bridgeOptions,
		ResponseInterval:    1000 * time.Millisecond,
		// TODO create a default config
	}, c.uplink, c.controller.GetEventChannel())

	// start the connection adapter
	err := connectionAdapter.Start()
	if err != nil {
		fmt.Printf("error starting connection adapter: %s\n", err)
		return
	}
	c.controller.AddConnection(header.CID, connectionAdapter)
}
