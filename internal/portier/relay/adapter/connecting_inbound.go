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

type connectingInboundState struct {
	// options are the connection adapter options
	options ConnectionAdapterOptions

	// eventChannel is the event channel
	eventChannel chan AdapterEvent

	// encoderDecoder is the encoder/decoder for msgpack
	encoderDecoder encoder.EncoderDecoder

	// uplink is the uplink
	uplink uplink.Uplink

	// ticker is the ticker
	ticker *time.Ticker

	// forwarder is the forwarder
	forwarder Forwarder
}

func (c *connectingInboundState) Start() error {
	// the connection open message has already been received, so try to dial the service and send the connection accept/failed message
	url := c.options.BridgeOptions.URLRemote
	var network string
	if url.Scheme == "udp" {
		network = "udp"
	} else {
		network = "tcp"
	}
	conn, err := net.Dial(network, url.Hostname()+":"+url.Port())
	if err != nil {
		mainError := fmt.Errorf("error dialing service: %s", err)
		// send connection failed message

		connectionFailedMessagePayload, _ := c.encoderDecoder.EncodeConnectionFailedMessage(messages.ConnectionFailedMessage{
			Reason: "error dialing service: " + err.Error(),
		})

		msg := messages.Message{
			Header: messages.MessageHeader{
				From: c.options.LocalDeviceId,
				To:   c.options.PeerDeviceId,
				Type: messages.CF,
				CID:  c.options.ConnectionId,
			},
			Message: connectionFailedMessagePayload,
		}
		// send the message to the uplink once, since we do not expect a response
		err = c.uplink.Send(msg)
		if err != nil {
			return fmt.Errorf("%s\nerror sending connection failed message: %s", mainError, err)
		}
		return mainError
	}

	connectionAcceptMessagePayload, _ := c.encoderDecoder.EncodeConnectionAcceptMessage(messages.ConnectionAcceptMessage{
		PCKey: c.options.LocalPublicKey,
	})

	msg := messages.Message{
		Header: messages.MessageHeader{
			From: c.options.LocalDeviceId,
			To:   c.options.PeerDeviceId,
			Type: messages.CA,
			CID:  c.options.ConnectionId,
		},
		Message: connectionAcceptMessagePayload,
	}
	if err != nil {
		return err
	}

	c.uplink.Send(msg)

	// send the message to the uplink using the ticker
	c.ticker = time.NewTicker(c.options.ResponseInterval)
	go func() {
		for range c.ticker.C {
			err := c.uplink.Send(msg)
			if err != nil {
				fmt.Printf("error sending connection accept message: %s\n", err)
			}
		}
	}()

	peerDevicePubKey := c.options.PeerDevicePublicKey
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
	c.forwarder = NewForwarder(forwarderOptions, conn, c.uplink, encryption, c.eventChannel)

	return nil
}

func (c *connectingInboundState) Stop() error {
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
	c.uplink.Send(msg)
	return c.forwarder.Close()
}

func (c *connectingInboundState) HandleMessage(msg messages.Message) (ConnectionAdapterState, error) {
	// if message is a data message, return connected state
	if msg.Header.Type == messages.D || msg.Header.Type == messages.CR {

		// TODO check signature

		c.ticker.Stop()
		return NewConnectedState(c.options, c.eventChannel, c.uplink, c.forwarder), nil
	}
	if msg.Header.Type == messages.CC {
		c.ticker.Stop()
		c.forwarder.Close()
		return nil, nil
	}
	if msg.Header.Type == messages.CO {
		return nil, nil
	}
	return nil, fmt.Errorf("expected message type [%s|%s], but got %s", messages.D, messages.CO, msg.Header.Type)
}

func NewConnectingInboundState(options ConnectionAdapterOptions, eventChannel chan AdapterEvent, uplink uplink.Uplink) ConnectionAdapterState {
	return &connectingInboundState{
		options:        options,
		eventChannel:   eventChannel,
		encoderDecoder: encoder.NewEncoderDecoder(),
		uplink:         uplink,
	}
}
