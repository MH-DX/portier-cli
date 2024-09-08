package adapter

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
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

	// conn is the connection
	conn net.Conn

	// context is the context
	context context.Context

	// stop is the context's cancel function
	stop context.CancelFunc
}

func (c *connectingOutboundState) Start() error {
	// send connection open message
	connectionOpenMessagePayload, err := c.encoderDecoder.EncodeConnectionOpenMessage(messages.ConnectionOpenMessage{
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
	ticker := time.NewTicker(c.options.ResponseInterval)

	go func() {
		defer ticker.Stop()
		for {
			err := c.uplink.Send(msg)
			if err != nil {
				// send error event
				c.eventChannel <- AdapterEvent{
					ConnectionId: c.options.ConnectionId,
					Type:         Error,
					Message:      err.Error(),
				}
			}
			select {
			case <-c.context.Done():
				log.Printf("outbound connection ticker %s closed\n", c.options.ConnectionId)
				return
			case <-ticker.C:
				continue
			}
		}
	}()

	return nil
}

func (c *connectingOutboundState) Stop() error {
	c.stop()
	return nil
}

func (c *connectingOutboundState) Close() error {
	c.stop()
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
	// if message is a connection accept message return connected state
	if msg.Header.Type == messages.CA {
		connectionAcceptMessage, err := c.encoderDecoder.DecodeConnectionAcceptMessage(msg.Message)
		if err != nil {
			return nil, err
		}
		log.Printf("connection accept message received: %v\n", connectionAcceptMessage)

		forwarderOptions := ForwarderOptions{
			Throughput:     c.options.ThroughputLimit,
			LocalDeviceID:  c.options.LocalDeviceId,
			PeerDeviceID:   c.options.PeerDeviceId,
			ConnectionID:   c.options.ConnectionId,
			ReadTimeout:    c.options.ConnectionReadTimeout,
			ReadBufferSize: c.options.ReadBufferSize,
		}
		forwarder := NewForwarder(forwarderOptions, c.conn, c.uplink, c.eventChannel)

		return NewConnectedState(c.options, c.eventChannel, c.uplink, forwarder), nil
	}
	if msg.Header.Type == messages.CF {
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
	ctx, stop := context.WithCancel(context.Background())
	return &connectingOutboundState{
		options:        options,
		eventChannel:   eventChannel,
		encoderDecoder: encoder.NewEncoderDecoder(),
		uplink:         uplink,
		conn:           conn,
		context:        ctx,
		stop:           stop,
	}
}
