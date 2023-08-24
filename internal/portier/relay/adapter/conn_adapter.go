package adapter

import (
	"fmt"
	"net"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/encryption"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"

	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
)

type ConnectionAdapter interface {
	// Start starts the connection
	Start() error

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

	// messageQueue is the queue of messages that are sent to the connection
	messageQueue chan messages.Message
}

type ConnectionMode string

const (
	// Inbound is the inbound connection mode, i.e. the connection is bridged to this relay
	Inbound ConnectionMode = "inbound"

	// Outbound is the outbound connection mode, i.e. the connection is bridged from this relay
	Outbound ConnectionMode = "outbound"
)

// NewConnectionAdapter creates a new connection adapter for an outbound connection
func NewOutboundConnectionAdapter(options ConnectionAdapterOptions, connection net.Conn, encoderDecoder encoder.EncoderDecoder, uplink uplink.Uplink) ConnectionAdapter {
	return &connectionAdapter{
		options:        options,
		encoderDecoder: encoderDecoder,
		uplink:         uplink,
		encryption:     nil,
		state:          NewConnectingOutboundState(options, encoderDecoder, uplink, connection, 1000),
		mode:           Outbound,
		messageQueue:   make(chan messages.Message, 1000),
	}
}

// NewConnectionAdapter creates a new connection adapter for an inbound connection
func NewInboundConnectionAdapter(options ConnectionAdapterOptions, encoderDecoder encoder.EncoderDecoder, uplink uplink.Uplink) ConnectionAdapter {
	return &connectionAdapter{
		options:        options,
		encoderDecoder: encoderDecoder,
		uplink:         uplink,
		encryption:     nil,
		state:          NewConnectingInboundState(options, encoderDecoder, uplink, 1000),
		mode:           Inbound,
		messageQueue:   make(chan messages.Message, 1000),
	}
}

// Start starts the connection adapter
func (c *connectionAdapter) Start() error {
	// start the connection adapter
	c.state.Start()

	go func() {
		// as long as the message queue is not closed, send messages from the queue
		for msg := range c.messageQueue {
			err := c.doSend(msg)
			if err != nil {
				fmt.Printf("error sending message: %v\n", err)
				return
			}
		}
	}()

	return nil
}

// Stop stops the connection adapter
func (c *connectionAdapter) Stop() error {
	// stop the connection adapter
	close(c.messageQueue)
	c.state.Stop()
	return nil
}

// Send sends a message to the queue
func (c *connectionAdapter) Send(msg messages.Message) error {
	// if the message queue is not closed, send the message to the message queue
	c.messageQueue <- msg
	return nil
}

// doSend sends a message to the current adapter state
func (c *connectionAdapter) doSend(msg messages.Message) error {
	newState, err := c.state.HandleMessage(msg)
	if err != nil {
		fmt.Printf("error handling message: %v\n", err)
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
			return err
		}
	}
	return nil
}
