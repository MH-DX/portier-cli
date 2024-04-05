package adapter

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
)

type connectedState struct {
	// options are the connection adapter options
	options ConnectionAdapterOptions

	// encoderDecoder is the encoder/decoder for msgpack
	encoderDecoder encoder.EncoderDecoder

	// forwader is the forwarder
	forwarder Forwarder

	// uplink is the uplink
	uplink uplink.Uplink

	// eventChannel is the event channel
	eventChannel chan<- AdapterEvent

	// context is the context
	context context.Context

	// stop is the context's cancel function
	stop context.CancelFunc
}

func (c *connectedState) Start() error {
	// start reading from the connection
	err := c.forwarder.Start()
	if err != nil {
		return err
	}

	msg := messages.Message{
		Header: messages.MessageHeader{
			From: c.options.LocalDeviceId,
			To:   c.options.PeerDeviceId,
			Type: messages.CR,
			CID:  c.options.ConnectionId,
		},
		Message: []byte{},
	}

	ticker := time.NewTicker(1000 * time.Millisecond)

	go func() {
		for {
			err := c.uplink.Send(msg)
			if err != nil {
				log.Printf("error sending CR message: %s\n", err)
				c.eventChannel <- AdapterEvent{
					ConnectionId: c.options.ConnectionId,
					Type:         Error,
					Message:      fmt.Sprintf("error sending CR message: %s", err),
				}
			}
			select {
			case <-c.context.Done():
				log.Printf("CR ticker %s closed\n", c.options.ConnectionId)
				return
			case <-ticker.C:
				continue
			}
		}
	}()

	return nil
}

func (c *connectedState) Stop() error {
	c.stop()
	return nil
}

func (c *connectedState) Close() error {
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
	return c.forwarder.Close()
}

func (c *connectedState) HandleMessage(msg messages.Message) (ConnectionAdapterState, error) {
	// decrypt the data
	if msg.Header.Type == messages.D {
		c.stop()
		err := c.forwarder.SendAsync(msg)
		if err != nil {
			c.eventChannel <- AdapterEvent{
				ConnectionId: c.options.ConnectionId,
				Type:         Error,
				Message:      fmt.Sprintf("error sending data message to forwarder: %s", err),
			}
			return nil, err
		}
		return nil, nil
	} else if msg.Header.Type == messages.DA {
		c.stop()
		// encode the data
		ackMessage, err := c.encoderDecoder.DecodeDataAckMessage(msg.Message)
		if err != nil {
			return nil, err
		}
		err = c.forwarder.Ack(ackMessage.Seq, ackMessage.Re)
		if err != nil {
			log.Printf("error acknowledging message: %s\n", err)
		}
		return nil, nil
	} else if msg.Header.Type == messages.CR {
		c.stop()
		return nil, nil
	} else if msg.Header.Type == messages.CA {
		return nil, nil
	} else if msg.Header.Type == messages.CC {
		c.eventChannel <- AdapterEvent{
			ConnectionId: c.options.ConnectionId,
			Type:         Closed,
			Message:      "connection closed by peer",
		}
		return nil, nil
	} else if msg.Header.Type == messages.NF {
		c.eventChannel <- AdapterEvent{
			ConnectionId: c.options.ConnectionId,
			Type:         Error,
			Message:      "connection not found by peer",
		}
		return nil, nil
	}
	return nil, fmt.Errorf("expected message type [%s|%s|%s|%s|%s], but got %s", messages.D, messages.DA, messages.CC, messages.CR, messages.NF, msg.Header.Type)
}

func NewConnectedState(options ConnectionAdapterOptions, eventChannel chan<- AdapterEvent, uplink uplink.Uplink, forwarder Forwarder) ConnectionAdapterState {
	ctx, stop := context.WithCancel(context.Background())
	return &connectedState{
		options:        options,
		eventChannel:   eventChannel,
		encoderDecoder: encoder.NewEncoderDecoder(),
		uplink:         uplink,
		forwarder:      forwarder,
		context:        ctx,
		stop:           stop,
	}
}
