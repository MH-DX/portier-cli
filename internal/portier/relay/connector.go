package relay

import "github.com/marinator86/portier-cli/internal/portier/relay/messages"

type connector struct {
	uplink Uplink

	encoderDecoder EncoderDecoder

	router Router
}

// NewConnector creates a new connector
func NewConnector(uplink Uplink, encoderDecoder EncoderDecoder, router Router) Connector {
	return &connector{
		uplink:         uplink,
		encoderDecoder: encoderDecoder,
		router:         router,
	}
}

// CreateInboundConnection creates an inbound connection
func (c *connector) CreateInboundConnection(header messages.MessageHeader, bridgeOptions messages.BridgeOptions, pcKey string) error {
	// create a new inbound connection adapter
	connectionAdapter := NewInboundConnectionAdapter(ConnectionAdapterOptions{
		ConnectionId:        header.CID,
		PeerDeviceId:        header.From,
		PeerDevicePublicKey: pcKey,
		BridgeOptions:       bridgeOptions,
	}, c.encoderDecoder, c.router, c.uplink)

	// start the connection adapter
	err := connectionAdapter.Start()
	if err != nil {
		return err
	}

	return nil
}
