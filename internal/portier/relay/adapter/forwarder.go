package adapter

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/mh-dx/portier-cli/internal/portier/relay/encoder"
	"github.com/mh-dx/portier-cli/internal/portier/relay/messages"
	"github.com/mh-dx/portier-cli/internal/portier/relay/uplink"
)

type ForwarderOptions struct {
	// Throughput is the maximum throughput in bytes per second
	Throughput int

	// LocalDeviceId is the id of the local device
	LocalDeviceID uuid.UUID

	// PeerDeviceId is the id of the peer device that this connection is bridged to/from
	PeerDeviceID uuid.UUID

	// ConnectionId is the connection id
	ConnectionID messages.ConnectionID

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
	SendAsync(msg messages.Message) error

	// Ack acknowledges a message, returns an error if the message is not found
	Ack(seqNo uint64, re bool) error

	// Stop stops the forwarder, and closes the send channel and the underlying connection
	Close() error
}

// NewForwarder creates a new forwarder.
func NewForwarder(options ForwarderOptions, conn net.Conn, uplink uplink.Uplink, eventChannel chan<- AdapterEvent) Forwarder {
	forwarderContext, cancel := context.WithCancel(context.Background())
	return &forwarder{
		options:        options,
		encoderDecoder: encoder.NewEncoderDecoder(),
		conn:           conn,
		uplink:         uplink,
		sendChannel:    make(chan messages.Message, 500),
		eventChannel:   eventChannel,
		window:         NewWindow(forwarderContext, NewDefaultWindowOptions(), uplink, encoder.NewEncoderDecoder()),
		messageHeap:    NewMessageHeap(NewDefaultMessageHeapOptions()),
		cancel:         cancel,
		context:        forwarderContext,
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

	// sendChannel is the channel to send messages to the forwarder
	sendChannel chan messages.Message

	// eventChannel is the event channel
	eventChannel chan<- AdapterEvent

	// window is the sliding window of pre-sent messages
	window Window

	// messageHeap is the message heap to buffer messages until they can be sent to the socket
	messageHeap MessageHeap

	// cancel is the cancel function for the context to stop the rto heap
	cancel context.CancelFunc

	// context is the context for the forwarder
	context context.Context
}

// Start starts the forwarder, returns a channel to which messages can be sent.
func (f *forwarder) Start() error {
	go func() {
		defer close(f.sendChannel)
		for {
			select {
			case msg, _ := <-f.sendChannel:
				// decode the data
				dm, err := f.encoderDecoder.DecodeDataMessage(msg.Message)
				if err != nil {
					f.eventChannel <- createEvent(Error, f.options.ConnectionID, "error decoding data message. Exiting", err)
					return
				}

				messages, err := f.messageHeap.Test(dm)
				if err != nil {
					if err.Error() == "old_message" || err.Error() == "duplicate_message" {
						err := f.ackMessage(dm.Seq, dm.Re)
						if err != nil {
							f.eventChannel <- createEvent(Error, f.options.ConnectionID, "error sending ack to uplink. Exiting", err)
							return
						}
						continue
					}
					f.eventChannel <- createEvent(Error, f.options.ConnectionID, "error in messageHeap. Exiting", err)
					return
				}

				if messages == nil {
					continue
				}

				for _, msg := range messages {
					_, err = f.conn.Write(msg.Data)
					if err != nil {
						f.eventChannel <- createEvent(Error, f.options.ConnectionID, "error processing message: ", err)
						break
					}
					err := f.ackMessage(msg.Seq, msg.Re)
					if err != nil {
						f.eventChannel <- createEvent(Error, f.options.ConnectionID, "error processing message: ", err)
						break
					}
				}

			case <-f.context.Done():
				log.Printf("forwarder stopped downward loop\n")
				return
			}
		}
	}()

	go func() {
		var seq uint64

		for {
			// exit if the context is done
			select {
			case <-f.context.Done():
				log.Printf("forwarder stopped upward loop\n")
				return
			default:
			}

			// read from the connection
			buf := make([]byte, f.options.ReadBufferSize)
			_ = f.conn.SetReadDeadline(time.Now().Add(f.options.ReadTimeout))
			n, err := f.conn.Read(buf)
			if err != nil {
				// if connection is closed, exit
				if err.Error() == "EOF" {
					f.eventChannel <- createEvent(Closed, f.options.ConnectionID, "connection closed by peer. Exiting", nil)
					return
				}
				// if timeout, continue
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.Printf("error reading from connection: %s\n", err)
				f.eventChannel <- createEvent(Error, f.options.ConnectionID, "error reading from connection. Exiting", err)
				return
			}
			if n == 0 {
				continue
			}
			// decrypt the data
			header := messages.MessageHeader{
				From: f.options.LocalDeviceID,
				To:   f.options.PeerDeviceID,
				Type: messages.D,
				CID:  f.options.ConnectionID,
			}
			dm := messages.DataMessage{
				Seq:  seq,
				Data: buf[:n],
			}
			seq++
			dmBytes, err := f.encoderDecoder.EncodeDataMessage(dm)
			if err != nil {
				log.Printf("error encoding data message: %s\n", err)
				f.eventChannel <- createEvent(Error, f.options.ConnectionID, "error encoding data message. Exiting", err)
				return
			}
			// wrap the data in a message
			msg := messages.Message{
				Header:  header,
				Message: dmBytes,
			}
			// send the data to the window
			err = f.window.add(msg, dm.Seq)
			if err != nil {
				log.Printf("error sending message to uplink: %s\n", err)
				f.eventChannel <- createEvent(Error, f.options.ConnectionID, "error sending message to uplink. Exiting", err)
				return
			}
		}
	}()

	return nil
}

func createEvent(eventType EventType, cid messages.ConnectionID, msg string, err error) AdapterEvent {
	return AdapterEvent{
		ConnectionId: cid,
		Type:         eventType,
		Message:      msg,
		Error:        err,
	}
}

// AsyncSend sends a message asynchronously, returns an error if the send buffer is full.
func (f *forwarder) SendAsync(msg messages.Message) error {
	select {
	case f.sendChannel <- msg:
	default:
		log.Printf("send buffer for %s full, dropping message\n", f.options.ConnectionID)
	}
	return nil
}

// Ack acknowledges a message.
func (f *forwarder) Ack(seqNo uint64, re bool) error {
	return f.window.ack(seqNo, re)
}

// Stop stops the forwarder, and closes the channel and the underlying connection.
func (f *forwarder) Close() error {
	select {
	case <-f.context.Done():
		log.Printf("forwarder for %s already stopped\n", f.options.ConnectionID)
		return nil
	default:
	}

	f.cancel()
	return f.conn.Close()
}

func (f *forwarder) ackMessage(seq uint64, re bool) error {
	ackMsg := messages.DataAckMessage{
		Seq: seq,
		Re:  re,
	}
	ackMsgBytes, _ := f.encoderDecoder.EncodeDataAckMessage(ackMsg)

	msg := messages.Message{
		Header: messages.MessageHeader{
			From: f.options.LocalDeviceID,
			To:   f.options.PeerDeviceID,
			Type: messages.DA,
			CID:  f.options.ConnectionID,
		},
		Message: ackMsgBytes,
	}

	_ = f.uplink.Send(msg)
	return nil
}
