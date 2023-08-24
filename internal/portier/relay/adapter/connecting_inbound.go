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

	// encoderDecoder is the encoder/decoder for msgpack
	encoderDecoder encoder.EncoderDecoder

	// uplink is the uplink
	uplink uplink.Uplink

	// responseInterval is the interval in which the connection accept/failed message is sent
	responseInterval time.Duration

	// ticker is the ticker
	ticker *time.Ticker

	// conn is the connection
	conn net.Conn

	// encryption is the encryptor/decryptor for this connection
	encryption encryption.Encryption
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

	localPubKey := "" // TODO: create the local key pair
	localPrivKey := ""

	connectionAcceptMessagePayload, _ := c.encoderDecoder.EncodeConnectionAcceptMessage(messages.ConnectionAcceptMessage{
		PCKey: localPubKey,
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
	c.ticker = time.NewTicker(c.responseInterval)
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
	c.encryption = encryption.NewEncryption(localPubKey, localPrivKey, peerDevicePubKey, cipher, curve)
	c.conn = conn

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
	return c.conn.Close()
}

func (c *connectingInboundState) HandleMessage(msg messages.Message) (ConnectionAdapterState, error) {
	// if message is a data message, return connected state
	if msg.Header.Type == messages.D {

		// TODO check signature

		c.ticker.Stop()
		return NewConnectedState(c.options, c.conn, c.encoderDecoder, c.uplink, c.encryption), nil
	}
	if msg.Header.Type == messages.CC {
		c.ticker.Stop()
		c.conn.Close()
		return nil, nil
	}
	if msg.Header.Type == messages.CO {
		return nil, nil
	}
	return nil, fmt.Errorf("expected message type [%s|%s], but got %s", messages.D, messages.CO, msg.Header.Type)
}

func NewConnectingInboundState(options ConnectionAdapterOptions, encoderDecoder encoder.EncoderDecoder, uplink uplink.Uplink, responseInterval time.Duration) ConnectionAdapterState {
	return &connectingInboundState{
		options:          options,
		encoderDecoder:   encoderDecoder,
		uplink:           uplink,
		responseInterval: responseInterval,
	}
}
