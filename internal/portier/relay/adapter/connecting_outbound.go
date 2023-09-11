package adapter

import (
	"fmt"
	"net"
	"time"

	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/encryption"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
)

type connectingOutboundState struct {
	// options are the connection adapter options
	options ConnectionAdapterOptions

	// eventChannel is the event channel
	eventChannel chan<- AdapterEvent

	// encoderDecoder is the encoder/decoder for msgpack
	encoderDecoder encoder.EncoderDecoder

	// uplink is the uplink
	uplink uplink.Uplink

	// ticker is the ticker
	ticker *time.Ticker

	// conn is the connection
	conn net.Conn
}

func (c *connectingOutboundState) Start() error {

	// send connection open message
	connectionOpenMessagePayload, err := c.encoderDecoder.EncodeConnectionOpenMessage(messages.ConnectionOpenMessage{
		PCKey:         c.options.LocalPublicKey,
		BridgeOptions: c.options.BridgeOptions,
	})
	if err != nil {
		return err
	}
	msg := messages.Message{
		Header: messages.MessageHeader{
			From: c.options.LocalDeviceId,
			To:   c.options.PeerDeviceId,
			Type: messages.CO,
			CID:  c.options.ConnectionId,
		},
		Message: connectionOpenMessagePayload,
	}

	// send the message to the uplink using the ticker
	c.ticker = time.NewTicker(c.options.ResponseInterval)

	err = c.uplink.Send(msg)
	if err != nil {
		return err
	}

	go func() {
		for range c.ticker.C {
			err := c.uplink.Send(msg)
			if err != nil {
				// send error event
				c.eventChannel <- AdapterEvent{
					ConnectionId: c.options.ConnectionId,
					Type:         Error,
					Message:      err.Error(),
				}
			}
		}
	}()

	return nil
}

func (c *connectingOutboundState) Stop() error {
	c.ticker.Stop()
	// send connection close message
	msg := messages.Message{
		Header: messages.MessageHeader{
			From: c.options.LocalDeviceId,
			To:   c.options.PeerDeviceId,
			Type: messages.CC,
			CID:  c.options.ConnectionId,
		},
		Message: []byte{},
	}
	_ = c.uplink.Send(msg)
	return c.conn.Close()
}

func (c *connectingOutboundState) HandleMessage(msg messages.Message) (ConnectionAdapterState, error) {
	// if message is a connection accept message, create encryption and return connected state
	if msg.Header.Type == messages.CA {

		// TODO check signature

		c.ticker.Stop()
		connectionAcceptMessage, err := c.encoderDecoder.DecodeConnectionAcceptMessage(msg.Message)
		if err != nil {
			return nil, err
		}

		peerDevicePubKey := connectionAcceptMessage.PCKey
		cipher := encryption.Cipher(c.options.BridgeOptions.Cipher)
		curve := encryption.Curve(c.options.BridgeOptions.Curve)
		encryption := encryption.NewEncryption(c.options.LocalPublicKey, c.options.LocalPrivateKey, peerDevicePubKey, cipher, curve)

		forwarderOptions := ForwarderOptions{
			Throughput:     c.options.ThroughputLimit,
			LocalDeviceId:  c.options.LocalDeviceId,
			PeerDeviceId:   c.options.PeerDeviceId,
			ConnectionId:   c.options.ConnectionId,
			ReadTimeout:    c.options.ConnectionReadTimeout,
			ReadBufferSize: c.options.ReadBufferSize,
		}
		forwarder := NewForwarder(forwarderOptions, c.conn, c.uplink, encryption, c.eventChannel)

		return NewConnectedState(c.options, c.eventChannel, c.uplink, forwarder), nil
	}
	if msg.Header.Type == messages.CF {
		c.ticker.Stop()
		c.conn.Close()
		connectionFailedMessage, err := c.encoderDecoder.DecodeConnectionFailedMessage(msg.Message)
		if err != nil {
			return nil, err
		}
		// send connection failed event
		c.eventChannel <- AdapterEvent{
			ConnectionId: c.options.ConnectionId,
			Type:         Error,
			Message:      connectionFailedMessage.Reason,
		}
		return nil, nil
	}
	if msg.Header.Type == messages.CC {
		c.ticker.Stop()
		c.conn.Close()
		c.eventChannel <- AdapterEvent{
			ConnectionId: c.options.ConnectionId,
			Type:         Closed,
			Message:      "connection closed",
		}
		return nil, nil
	}
	return nil, fmt.Errorf("expected message type [%s|%s|%s], but got %s", messages.CA, messages.CF, messages.CC, msg.Header.Type)
}

func NewConnectingOutboundState(options ConnectionAdapterOptions, eventChannel chan<- AdapterEvent, uplink uplink.Uplink, conn net.Conn) ConnectionAdapterState {
	return &connectingOutboundState{
		options:        options,
		eventChannel:   eventChannel,
		encoderDecoder: encoder.NewEncoderDecoder(),
		uplink:         uplink,
		conn:           conn,
	}
}
