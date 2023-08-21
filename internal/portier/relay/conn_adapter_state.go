package relay

import (
	"fmt"
	"net"
	"time"

	"github.com/marinator86/portier-cli/internal/portier/relay/encryption"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
)

type connectingInboundState struct {
	// options are the connection adapter options
	options ConnectionAdapterOptions

	// encoderDecoder is the encoder/decoder for msgpack
	encoderDecoder EncoderDecoder

	// uplink is the uplink
	uplink Uplink

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

func NewConnectingInboundState(options ConnectionAdapterOptions, encoderDecoder EncoderDecoder, uplink Uplink, responseInterval time.Duration) ConnectionAdapterState {
	return &connectingInboundState{
		options:          options,
		encoderDecoder:   encoderDecoder,
		uplink:           uplink,
		responseInterval: responseInterval,
	}
}

// =====================================================================================================================

type connectingOutboundState struct {
	// options are the connection adapter options
	options ConnectionAdapterOptions

	// encoderDecoder is the encoder/decoder for msgpack
	encoderDecoder EncoderDecoder

	// uplink is the uplink
	uplink Uplink

	// responseInterval is the interval in which the connection accept/failed message is sent
	responseInterval time.Duration

	// ticker is the ticker
	ticker *time.Ticker

	// conn is the connection
	conn net.Conn

	localPubKey string

	localPrivKey string
}

func (c *connectingOutboundState) Start() error {

	c.localPubKey = "" // TODO: create the local key pair
	c.localPrivKey = ""

	// send connection open message
	connectionOpenMessagePayload, err := c.encoderDecoder.EncodeConnectionOpenMessage(messages.ConnectionOpenMessage{
		PCKey:         c.localPubKey,
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
	c.ticker = time.NewTicker(c.responseInterval)
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
		encryption := encryption.NewEncryption(c.localPubKey, c.localPrivKey, peerDevicePubKey, cipher, curve)

		return NewConnectedState(c.options, c.conn, c.encoderDecoder, c.uplink, encryption), nil
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
	return nil, fmt.Errorf("expected message type [%s|%s], but got %s", messages.CA, messages.CF, msg.Header.Type)
}

func NewConnectingOutboundState(options ConnectionAdapterOptions, encoderDecoder EncoderDecoder, uplink Uplink, conn net.Conn, responseInterval time.Duration) ConnectionAdapterState {
	return &connectingOutboundState{
		options:          options,
		encoderDecoder:   encoderDecoder,
		uplink:           uplink,
		responseInterval: responseInterval,
		conn:             conn,
	}
}

// =====================================================================================================================

type connectedState struct {
	// options are the connection adapter options
	options ConnectionAdapterOptions

	// encoderDecoder is the encoder/decoder for msgpack
	encoderDecoder EncoderDecoder

	// uplink is the uplink
	uplink Uplink

	// conn is the connection
	conn net.Conn

	// encryption is the encryptor/decryptor for this connection
	encryption encryption.Encryption
}

func (c *connectedState) Start() error {
	// start reading from the connection
	errorChannel := make(chan error)
	go func(e chan error) {
		for {

			// TODO wait for acks

			// read from the connection
			buf := make([]byte, 1024)
			n, err := c.conn.Read(buf)
			if err != nil {
				fmt.Printf("error reading from connection: %s\n", err)
				e <- err
				return
			}
			// decrypt the data
			header := messages.MessageHeader{
				From: c.options.LocalDeviceId,
				To:   c.options.PeerDeviceId,
				Type: messages.D,
				CID:  c.options.ConnectionId,
			}
			encrypted, err := c.encryption.Encrypt(header, buf[:n])
			if err != nil {
				fmt.Printf("error encrypting data: %s\n", err)
				e <- err
				return
			}
			// wrap the data in a message
			msg := messages.Message{
				Header:  header,
				Message: encrypted,
			}
			// send the data to the uplink
			err = c.uplink.Send(msg)
			if err != nil {
				fmt.Printf("error sending message to uplink: %s\n", err)
				e <- err
				return
			}
		}
	}(errorChannel)

	// TODO use error channel

	return nil
}

func (c *connectedState) Stop() error {
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

func (c *connectedState) HandleMessage(msg messages.Message) (ConnectionAdapterState, error) {
	// decrypt the data
	if msg.Header.Type == messages.D {
		decrypted, err := c.encryption.Decrypt(msg.Header, msg.Message)
		if err != nil {
			return nil, err
		}
		// send the data to the connection
		_, err = c.conn.Write(decrypted)
		if err != nil {
			return nil, err
		}

		// TODO send acks

		return nil, nil
	} else if msg.Header.Type == messages.CC {
		c.conn.Close()
		return nil, nil
	}
	return nil, fmt.Errorf("expected message type [%s|%s], but got %s", messages.D, messages.CC, msg.Header.Type)
}

func NewConnectedState(options ConnectionAdapterOptions, conn net.Conn, encoderDecoder EncoderDecoder, uplink Uplink, encryption encryption.Encryption) ConnectionAdapterState {
	return &connectedState{
		options:        options,
		encoderDecoder: encoderDecoder,
		uplink:         uplink,
		conn:           conn,
		encryption:     encryption,
	}
}
