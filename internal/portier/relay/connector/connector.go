package connector

import (
	"github.com/marinator86/portier-cli/internal/portier/relay/adapter"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
)

type Connector interface {
	// CreateConnection creates a new connection
	CreateInboundConnection(header messages.MessageHeader, options messages.BridgeOptions, pcKey string) error
}

type connector struct {
	uplink uplink.Uplink

	encoderDecoder encoder.EncoderDecoder
}

// NewConnector creates a new connector
func NewConnector(uplink uplink.Uplink) Connector {
	return &connector{
		uplink:         uplink,
		encoderDecoder: encoder.NewEncoderDecoder(),
	}
}

// CreateInboundConnection creates an inbound connection
func (c *connector) CreateInboundConnection(header messages.MessageHeader, bridgeOptions messages.BridgeOptions, pcKey string) error {
	// create a new inbound connection adapter
	connectionAdapter := adapter.NewInboundConnectionAdapter(adapter.ConnectionAdapterOptions{
		ConnectionId:        header.CID,
		PeerDeviceId:        header.From,
		PeerDevicePublicKey: pcKey,
		BridgeOptions:       bridgeOptions,
	}, c.uplink)

	// start the connection adapter
	err := connectionAdapter.Start()
	if err != nil {
		return err
	}

	return nil
}
