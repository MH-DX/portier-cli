package adapter

import (
	"fmt"
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

	// CR ticker
	CRticker *time.Ticker

	// eventChannel is the event channel
	eventChannel chan<- AdapterEvent
}

func (c *connectedState) Start() error {
	// start reading from the connection
	err := c.forwarder.Start()
	if err != nil {
		return err
	}

	// start CR ticker
	c.CRticker = time.NewTicker(1000 * time.Millisecond)
	msg := messages.Message{
		Header: messages.MessageHeader{
			From: c.options.LocalDeviceId,
			To:   c.options.PeerDeviceId,
			Type: messages.CR,
			CID:  c.options.ConnectionId,
		},
		Message: []byte{},
	}

	err = c.uplink.Send(msg)
	if err != nil {
		return err
	}

	go func() {
		for range c.CRticker.C {
			err := c.uplink.Send(msg)
			if err != nil {
				fmt.Printf("error sending CR message: %s\n", err)
				c.eventChannel <- AdapterEvent{
					ConnectionId: c.options.ConnectionId,
					Type:         Error,
					Message:      fmt.Sprintf("error sending CR message: %s", err),
				}
			}
		}
	}()

	return nil
}

func (c *connectedState) Stop() error {
	c.CRticker.Stop()
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
		c.CRticker.Stop()
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
		c.CRticker.Stop()
		// encode the data
		ackMessage, err := c.encoderDecoder.DecodeDataAckMessage(msg.Message)
		if err != nil {
			return nil, err
		}
		_ = c.forwarder.Ack(ackMessage.Seq, ackMessage.Re)
		return nil, nil
	} else if msg.Header.Type == messages.CC {
		c.CRticker.Stop()
		err := c.forwarder.Close()
		return nil, err
	} else if msg.Header.Type == messages.CR {
		return nil, nil
	} else if msg.Header.Type == messages.CA {
		return nil, nil
	}
	return nil, fmt.Errorf("expected message type [%s|%s|%s|%s], but got %s", messages.D, messages.DA, messages.CC, messages.CR, msg.Header.Type)
}

func NewConnectedState(options ConnectionAdapterOptions, eventChannel chan<- AdapterEvent, uplink uplink.Uplink, forwarder Forwarder) ConnectionAdapterState {
	return &connectedState{
		options:        options,
		eventChannel:   eventChannel,
		encoderDecoder: encoder.NewEncoderDecoder(),
		uplink:         uplink,
		forwarder:      forwarder,
	}
}
