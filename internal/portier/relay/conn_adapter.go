package relay

import (
	"net"
)

type connectionAdapter struct {
	options ConnectionAdapterOptions

	// connection is the connection
	connection net.Conn

	// encoderDecoder is the encoder/decoder
	encoderDecoder EncoderDecoder

	// router is the router
	router Router

	// uplink is the uplink
	uplink Uplink

	// state is the state of the connection adapter
	state ConnectionAdapterState

	// Mode is either inbound or outbound
	mode ConnectionMode
}

type ConnectionAdapterState string

const (
	// Connecting is the state when the connection adapter has been started but the connection is not yet established
	Connecting ConnectionAdapterState = "connecting"

	// Connected is the state when the connection adapter has been started and the connection is established
	Connected ConnectionAdapterState = "connected"

	// Failed is the state when the connection adapter has been started but the connection could not be established
	Failed ConnectionAdapterState = "failed"

	// Closed is the state when the connection adapter has been stopped
	Closed ConnectionAdapterState = "closed"
)

type ConnectionMode string

const (
	// Inbound is the inbound connection mode, i.e. the connection is bridged to this relay
	Inbound ConnectionMode = "inbound"

	// Outbound is the outbound connection mode, i.e. the connection is bridged from this relay
	Outbound ConnectionMode = "outbound"
)

// NewConnectionAdapter creates a new connection adapter for an outbound connection
func NewOutboundConnectionAdapter(options ConnectionAdapterOptions, connection net.Conn, encoderDecoder EncoderDecoder, router Router, uplink Uplink) ConnectionAdapter {
	return &connectionAdapter{
		options:        options,
		connection:     connection,
		encoderDecoder: encoderDecoder,
		router:         router,
		uplink:         uplink,
		state:          Connecting,
		mode:           Outbound,
	}
}

// NewConnectionAdapter creates a new connection adapter for an inbound connection
func NewInboundConnectionAdapter(options ConnectionAdapterOptions, encoderDecoder EncoderDecoder, router Router, uplink Uplink) ConnectionAdapter {
	return &connectionAdapter{
		options:        options,
		connection:     nil,
		encoderDecoder: encoderDecoder,
		router:         router,
		uplink:         uplink,
		state:          Connecting,
		mode:           Inbound,
	}
}

// Start starts the connection adapter
func (c *connectionAdapter) Start() error {
	// start the connection adapter
	c.router.AddConnection(c.options.ConnectionId, c)
	return nil
}

// Stop stops the connection adapter
func (c *connectionAdapter) Stop() error {
	// stop the connection adapter
	c.router.RemoveConnection(c.options.ConnectionId)
	return nil
}

// Send sends a message
func (c *connectionAdapter) Send(msg []byte) error {
	return nil
}
