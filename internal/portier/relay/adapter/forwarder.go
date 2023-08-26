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

type ForwarderOptions struct {
	// Throughput is the maximum throughput in bytes per second
	Throughput int

	// LocalDeviceId is the id of the local device
	LocalDeviceId uuid.UUID

	// PeerDeviceId is the id of the peer device that this connection is bridged to/from
	PeerDeviceId uuid.UUID

	// ConnectionId is the connection id
	ConnectionId messages.ConnectionId
}

// Forwarder controls the flow of messages from and to spider.
// It is responsible to acknowledge messages and to process acks.
type Forwarder interface {
	// Start starts the forwarder, returns a channel to which messages can be sent
	Start() (chan messages.DataMessage, chan error, error)

	// Ack acknowledges a message
	Ack(seqNo uint64) error

	// Stop stops the forwarder, and closes the send channel and the underlying connection
	Close() error
}

// NewForwarder creates a new forwarder
func NewForwarder(options ForwarderOptions, conn net.Conn, uplink uplink.Uplink, encryption encryption.Encryption) Forwarder {
	return &forwarder{
		options:        options,
		encoderDecoder: encoder.NewEncoderDecoder(),
		conn:           conn,
		uplink:         uplink,
		encryption:     encryption,
	}
}

type forwarder struct {
	// options are the forwarder options
	options ForwarderOptions

	// encoderDecoder is the encoder/decoder for msgpack
	encoderDecoder encoder.EncoderDecoder

	// conn is the connection
	conn net.Conn

	// uplink is the uplink
	uplink uplink.Uplink

	// encryption is the encryptor/decryptor for this connection
	encryption encryption.Encryption

	// stopChannel is the channel to stop the forwarder
	stopChannel chan struct{}
}

// Start starts the forwarder, returns a channel to which messages can be sent
func (f *forwarder) Start() (chan messages.DataMessage, chan error, error) {
	sendChannel := make(chan messages.DataMessage)
	errorChannel := make(chan error)

	go func() {
		defer close(sendChannel)
		for {
			select {
			case <-f.stopChannel:
				return
			case msg := <-sendChannel:
				// decode the message
				dataMessage, err := f.encoderDecoder.DecodeDataMessage(msg.Data)
				if err != nil {
					errorChannel <- err
					return
				}
				_, err = f.conn.Write(dataMessage.Data)
				if err != nil {
					errorChannel <- err
					return
				}
			}
		}
	}()

	go func() {
		for {
			// exit if the stop channel is closed
			select {
			case <-f.stopChannel:
				return
			default:
			}

			// TODO wait for acks

			// read from the connection
			buf := make([]byte, 1024)
			n, err := f.conn.Read(buf)
			if err != nil {
				fmt.Printf("error reading from connection: %s\n", err)
				errorChannel <- err
				return
			}
			// decrypt the data
			header := messages.MessageHeader{
				From: f.options.LocalDeviceId,
				To:   f.options.PeerDeviceId,
				Type: messages.D,
				CID:  f.options.ConnectionId,
			}
			dm := messages.DataMessage{
				Seq:  0,
				Data: buf[:n],
			}
			dmBytes, err := f.encoderDecoder.EncodeDataMessage(dm)
			if err != nil {
				fmt.Printf("error encoding data message: %s\n", err)
				errorChannel <- err
				return
			}
			encrypted, err := f.encryption.Encrypt(header, dmBytes)
			if err != nil {
				fmt.Printf("error encrypting data: %s\n", err)
				errorChannel <- err
				return
			}
			// wrap the data in a message
			msg := messages.Message{
				Header:  header,
				Message: encrypted,
			}
			// send the data to the uplink
			err = f.uplink.Send(msg)
			if err != nil {
				fmt.Printf("error sending message to uplink: %s\n", err)
				errorChannel <- err
				return
			}
		}
	}()

	return sendChannel, errorChannel, nil
}

// Ack acknowledges a message
func (f *forwarder) Ack(seqNo uint64) error {
	// TODO
	return nil
}

// Stop stops the forwarder, and closes the channel and the underlying connection
func (f *forwarder) Close() error {
	if f.stopChannel != nil {
		close(f.stopChannel)
	}
	return f.conn.Close()
}
