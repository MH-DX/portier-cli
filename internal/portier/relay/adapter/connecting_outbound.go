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
	c.ticker = time.NewTicker(c.options.responseInterval)

	c.uplink.Send(msg)

	go func() {
		for range c.ticker.C {
			err := c.uplink.Send(msg)
			if err != nil {
				fmt.Printf("error sending connection open message: %s\n", err)
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
			Throughput:    1000,
			LocalDeviceId: c.options.LocalDeviceId,
			PeerDeviceId:  c.options.PeerDeviceId,
			ConnectionId:  c.options.ConnectionId,
		}
		forwarder := NewForwarder(forwarderOptions, c.conn, c.uplink, encryption)

		return NewConnectedState(c.options, c.uplink, forwarder), nil
	}
	if msg.Header.Type == messages.CF {
		c.ticker.Stop()
		c.conn.Close()
		connectionFailedMessage, err := c.encoderDecoder.DecodeConnectionFailedMessage(msg.Message)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("connection failed: %s", connectionFailedMessage.Reason)
	}
	if msg.Header.Type == messages.CC {
		c.ticker.Stop()
		c.conn.Close()
		return nil, nil
	}
	return nil, fmt.Errorf("expected message type [%s|%s|%s], but got %s", messages.CA, messages.CF, messages.CC, msg.Header.Type)
}

func NewConnectingOutboundState(options ConnectionAdapterOptions, uplink uplink.Uplink, conn net.Conn) ConnectionAdapterState {
	return &connectingOutboundState{
		options:        options,
		encoderDecoder: encoder.NewEncoderDecoder(),
		uplink:         uplink,
		conn:           conn,
	}
}
