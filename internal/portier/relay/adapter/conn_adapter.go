package adapter

import (
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/encryption"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"

	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
)

type AdapterEventType string

const (
	Closed AdapterEventType = "adapter-closed"
	Error  AdapterEventType = "error"
)

type AdapterEvent struct {
	Type    AdapterEventType
	Message string
	Error   error
}

type ConnectionAdapter interface {
	// Start starts the connection
	Start() (chan AdapterEvent, error)

	// Stop stops the connection
	Stop() error

	// Send sends a message to the connection
	Send(msg messages.Message) error
}

type ConnectionAdapterState interface {
	Start() error

	Stop() error

	HandleMessage(msg messages.Message) (ConnectionAdapterState, error)
}

type ConnectionAdapterOptions struct {
	// ConnectionId is the connection id
	ConnectionId messages.ConnectionId

	// LocalDeviceId is the id of the local device
	LocalDeviceId uuid.UUID

	// PeerDeviceId is the id of the peer device that this connection is bridged to/from
	PeerDeviceId uuid.UUID

	// PeerDevicePublicKey is the public key of the peer device that this connection is bridged to/from
	PeerDevicePublicKey string

	// BridgeOptions are the bridge options
	BridgeOptions messages.BridgeOptions

	// LocalPublicKey is the public key of the local device
	LocalPublicKey string

	// LocalPrivateKey is the private key of the local device
	LocalPrivateKey string

	// responseInterval is the interval in which the connection accept/failed message is sent
	ResponseInterval time.Duration

	// connectionReadTimeout is the read timeout for the connection
	ConnectionReadTimeout time.Duration

	// connectionThroughput is the throughput limit for the connection in bytes per second
	ThroughputLimit int
}

type connectionAdapter struct {
	options ConnectionAdapterOptions

	// encoderDecoder is the encoder/decoder for msgpack
	encoderDecoder encoder.EncoderDecoder

	// encryption is the encryptor/decryptor for this connection
	encryption encryption.Encryption

	// uplink is the uplink
	uplink uplink.Uplink

	// state is the current state of the connection adapter
	state ConnectionAdapterState

	// Mode is either inbound or outbound
	mode ConnectionMode

	// eventChannel is the channel that is used to send events to the caller
	eventChannel chan AdapterEvent
}

type ConnectionMode string

const (
	// Inbound is the inbound connection mode, i.e. the connection is bridged to this relay
	Inbound ConnectionMode = "inbound"

	// Outbound is the outbound connection mode, i.e. the connection is bridged from this relay
	Outbound ConnectionMode = "outbound"
)

// NewConnectionAdapter creates a new connection adapter for an outbound connection
func NewOutboundConnectionAdapter(options ConnectionAdapterOptions, connection net.Conn, uplink uplink.Uplink, eventChannel chan AdapterEvent) ConnectionAdapter {
	return &connectionAdapter{
		options:        options,
		encoderDecoder: encoder.NewEncoderDecoder(),
		uplink:         uplink,
		encryption:     nil,
		state:          NewConnectingOutboundState(options, eventChannel, uplink, connection),
		mode:           Outbound,
		eventChannel:   eventChannel,
	}
}

// NewConnectionAdapter creates a new connection adapter for an inbound connection
func NewInboundConnectionAdapter(options ConnectionAdapterOptions, uplink uplink.Uplink, eventChannel chan AdapterEvent) ConnectionAdapter {
	return &connectionAdapter{
		options:        options,
		encoderDecoder: encoder.NewEncoderDecoder(),
		uplink:         uplink,
		encryption:     nil,
		state:          NewConnectingInboundState(options, eventChannel, uplink),
		mode:           Inbound,
		eventChannel:   eventChannel,
	}
}

// Start starts the connection adapter
func (c *connectionAdapter) Start() (chan AdapterEvent, error) {
	// start the connection adapter
	c.state.Start()
	return c.eventChannel, nil
}

// Stop stops the connection adapter
func (c *connectionAdapter) Stop() error {
	// stop the connection adapter
	c.state.Stop()
	return nil
}

// Send sends a message to the queue
func (c *connectionAdapter) Send(msg messages.Message) error {
	// if the message queue is not closed, send the message to the message queue
	newState, err := c.state.HandleMessage(msg)
	if err != nil {
		fmt.Printf("error handling message: %v\n", err)
		c.Stop()
		return err
	}
	if newState != nil {
		c.state = newState
		err = newState.Start()
		if err != nil {
			fmt.Printf("error starting new state: %v\n", err)
			return err
		}
		err = c.Send(msg)
		if err != nil {
			fmt.Printf("error handling message: %v\n", err)
			c.Stop()
			return err
		}
	}
	return nil
}
