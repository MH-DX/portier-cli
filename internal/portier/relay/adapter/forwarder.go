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

type ForwarderOptions struct {
	// Throughput is the maximum throughput in bytes per second
	Throughput int

	// LocalDeviceId is the id of the local device
	LocalDeviceId uuid.UUID

	// PeerDeviceId is the id of the peer device that this connection is bridged to/from
	PeerDeviceId uuid.UUID

	// ConnectionId is the connection id
	ConnectionId messages.ConnectionId

	// ReadTimeout is the read timeout
	ReadTimeout time.Duration

	// ReadBufferSize is the size of the read buffer in bytes
	ReadBufferSize int
}

// Forwarder controls the flow of messages from and to spider.
// It is responsible to acknowledge messages and to process acks.
type Forwarder interface {
	// Start starts the forwarder, returns a channel to which messages can be sent
	Start() error

	// AsyncSend sends a message asynchronously, returns an error if the send buffer is full
	SendAsync(msg messages.DataMessage) error

	// Ack acknowledges a message, returns an error if the message is not found
	Ack(seqNo uint64) error

	// Stop stops the forwarder, and closes the send channel and the underlying connection
	Close() error
}

// NewForwarder creates a new forwarder
func NewForwarder(options ForwarderOptions, conn net.Conn, uplink uplink.Uplink, encryption encryption.Encryption, eventChannel chan<- AdapterEvent) Forwarder {
	return &forwarder{
		options:        options,
		encoderDecoder: encoder.NewEncoderDecoder(),
		conn:           conn,
		uplink:         uplink,
		encryption:     encryption,
		sendChannel:    make(chan messages.DataMessage, 1000000),
		stopChannel:    make(chan struct{}),
		eventChannel:   eventChannel,
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

	// sendChannel is the channel to send messages to the forwarder
	sendChannel chan messages.DataMessage

	// stopChannel is the channel to stop the forwarder
	stopChannel chan struct{}

	// eventChannel is the event channel
	eventChannel chan<- AdapterEvent
}

// Start starts the forwarder, returns a channel to which messages can be sent
func (f *forwarder) Start() error {
	go func() {
		defer close(f.sendChannel)
		for {
			select {
			case <-f.stopChannel:
				return
			case msg := <-f.sendChannel:
				// decode the message
				_, err := f.conn.Write(msg.Data)
				if err != nil {
					f.eventChannel <- createEvent(f.options.ConnectionId, "error writing to connection. Exiting", err)
					return
				}
			}
		}
	}()

	go func() {
		var seq uint64 = 0

		for {
			// exit if the stop channel is closed
			select {
			case <-f.stopChannel:
				return
			default:
			}

			// TODO wait for acks

			// read from the connection
			buf := make([]byte, f.options.ReadBufferSize)
			f.conn.SetReadDeadline(time.Now().Add(f.options.ReadTimeout))
			n, err := f.conn.Read(buf)
			if n == 0 {
				continue
			}
			if err != nil {
				// if connection is closed, exit
				if err.Error() == "EOF" {
					f.eventChannel <- createEvent(f.options.ConnectionId, "connection closed by peer. Exiting", nil)
					return
				}
				fmt.Printf("error reading from connection: %s\n", err)
				f.eventChannel <- createEvent(f.options.ConnectionId, "error reading from connection. Exiting", err)
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
				Seq:  seq,
				Data: buf[:n],
			}
			seq++
			dmBytes, err := f.encoderDecoder.EncodeDataMessage(dm)
			if err != nil {
				fmt.Printf("error encoding data message: %s\n", err)
				f.eventChannel <- createEvent(f.options.ConnectionId, "error encoding data message. Exiting", err)
				return
			}
			encrypted, err := f.encryption.Encrypt(header, dmBytes)
			if err != nil {
				fmt.Printf("error encrypting data: %s\n", err)
				f.eventChannel <- createEvent(f.options.ConnectionId, "error encrypting data. Exiting", err)
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
				f.eventChannel <- createEvent(f.options.ConnectionId, "error sending message to uplink. Exiting", err)
				return
			}
		}
	}()

	return nil
}

func createEvent(cid messages.ConnectionId, msg string, err error) AdapterEvent {
	return AdapterEvent{
		ConnectionId: cid,
		Type:         Error,
		Message:      msg,
		Error:        err,
	}
}

// AsyncSend sends a message asynchronously, returns an error if the send buffer is full
func (f *forwarder) SendAsync(msg messages.DataMessage) error {
	select {
	case <-f.stopChannel:
		return fmt.Errorf("forwarder is stopped")
	case f.sendChannel <- msg:
		return nil
	default:
		return fmt.Errorf("send buffer is full")
	}
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
