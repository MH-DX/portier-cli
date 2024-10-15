package adapter

import (
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/mh-dx/portier-cli/internal/portier/ptls"
	"github.com/mh-dx/portier-cli/internal/portier/relay/encoder"
	"github.com/mh-dx/portier-cli/internal/portier/relay/messages"
	"github.com/mh-dx/portier-cli/internal/portier/relay/uplink"
)

type EventType string

const (
	Closed EventType = "adapter-closed"
	Error  EventType = "error"
)

type AdapterEvent struct {
	ConnectionId messages.ConnectionID
	Type         EventType
	Message      string
	Error        error
}

type ConnectionAdapter interface {
	// Start starts the connection
	Start() error

	// Stop stops the connection
	Close() error

	// Send sends a message to the connection
	Send(msg messages.Message)
}

type ConnectionAdapterState interface {
	Start() error

	Stop() error

	Close() error

	HandleMessage(msg messages.Message) (ConnectionAdapterState, error)
}

type ConnectionAdapterOptions struct {
	// ConnectionID is the connection id
	ConnectionId messages.ConnectionID

	// LocalDeviceId is the id of the local device
	LocalDeviceId uuid.UUID

	// PeerDeviceId is the id of the peer device that this connection is bridged to/from
	PeerDeviceId uuid.UUID

	// BridgeOptions are the bridge options
	BridgeOptions messages.BridgeOptions

	// ResponseInterval is the interval in which the connection accept/failed message is sent
	ResponseInterval time.Duration

	// ConnectionReadTimeout is the read timeout for the connection
	ConnectionReadTimeout time.Duration

	// ThroughputLimit is the throughput limit for the connection in bytes per second
	ThroughputLimit int

	// ReadBufferSize is the size of the read buffer in bytes
	ReadBufferSize int
}

type connectionAdapter struct {
	options ConnectionAdapterOptions

	// encoderDecoder is the encoder/decoder for msgpack
	encoderDecoder encoder.EncoderDecoder

	// uplink is the uplink
	uplink uplink.Uplink

	// state is the current state of the connection adapter
	state ConnectionAdapterState

	// Mode is either inbound or outbound
	mode ConnectionMode

	// eventChannel is the channel that is used to send events to the caller
	eventChannel chan<- AdapterEvent
}

type ConnectionMode string

const (
	// Inbound is the inbound connection mode, i.e. the connection is bridged to this relay.
	Inbound ConnectionMode = "inbound"

	// Outbound is the outbound connection mode, i.e. the connection is bridged from this relay.
	Outbound ConnectionMode = "outbound"
)

// NewConnectionAdapter creates a new connection adapter for an outbound connection.
func NewOutboundConnectionAdapter(options ConnectionAdapterOptions, connection net.Conn, uplink uplink.Uplink, eventChannel chan<- AdapterEvent) ConnectionAdapter {
	return &connectionAdapter{
		options:        options,
		encoderDecoder: encoder.NewEncoderDecoder(),
		uplink:         uplink,
		state:          NewConnectingOutboundState(options, eventChannel, uplink, connection),
		mode:           Outbound,
		eventChannel:   eventChannel,
	}
}

// NewConnectionAdapter creates a new connection adapter for an inbound connection.
func NewInboundConnectionAdapter(options ConnectionAdapterOptions, uplink uplink.Uplink, eventChannel chan<- AdapterEvent, ptls ptls.PTLS) ConnectionAdapter {
	return &connectionAdapter{
		options:        options,
		encoderDecoder: encoder.NewEncoderDecoder(),
		uplink:         uplink,
		state:          NewConnectingInboundState(options, eventChannel, uplink, ptls),
		mode:           Inbound,
		eventChannel:   eventChannel,
	}
}

// Start starts the connection adapter.
func (c *connectionAdapter) Start() error {
	// start the connection adapter
	err := c.state.Start()
	if err != nil {
		return err
	}
	return nil
}

// Stop stops the connection adapter.
func (c *connectionAdapter) Close() error {
	// stop the connection adapter
	err := c.state.Close()
	if err != nil {
		return err
	}
	return nil
}

// Send sends a message to the queue.
func (c *connectionAdapter) Send(msg messages.Message) {
	// if the message queue is not closed, send the message to the message queue
	newState, err := c.state.HandleMessage(msg)
	if err != nil {
		fmt.Printf("error handling message: %v\n", err)
		return
	}
	if newState != nil {
		err := c.state.Stop()
		if err != nil {
			fmt.Printf("error stopping old state: %v\n", err)
		}
		c.state = newState
		err = newState.Start()
		if err != nil {
			fmt.Printf("error starting new state: %v\n", err)
			c.eventChannel <- AdapterEvent{
				ConnectionId: c.options.ConnectionId,
				Type:         Error,
				Error:        err,
			}
			return
		}
		c.Send(msg)
	}
}
