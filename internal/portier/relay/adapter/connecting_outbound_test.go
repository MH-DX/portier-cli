package adapter

import (
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestOutboundConnection(testing *testing.T) {
	// GIVEN
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	// Signals
	openChannel := make(chan bool, 1)
	closedChannel := make(chan bool, 1)
	eventChannel := make(chan AdapterEvent, 10)

	urlRemote, _ := url.Parse("tcp://localhost:" + fmt.Sprint(port))
	options := ConnectionAdapterOptions{
		ConnectionId:        "test-connection-id",
		LocalDeviceId:       uuid.New(),
		PeerDeviceId:        uuid.New(),
		PeerDevicePublicKey: "test-peer-device-public-key",
		ResponseInterval:    1000 * time.Millisecond,
		BridgeOptions: messages.BridgeOptions{
			URLRemote: *urlRemote,
		},
	}

	// mock uplink
	uplink := MockUplink{}

	uplink.On("Send", mock.MatchedBy(func(msg messages.Message) bool {
		if msg.Header.Type == messages.CO {
			openChannel <- true
		}
		if msg.Header.Type == messages.CC {
			closedChannel <- true
		}
		return true
	})).Return(nil)

	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	assert.Nil(testing, err)
	underTest := NewConnectingOutboundState(options, eventChannel, &uplink, conn)

	// WHEN
	_ = underTest.Start()

	// THEN
	<-openChannel // connection accepted message sent
	<-openChannel // resend connection accepted message sent
	_ = underTest.Close()
	<-closedChannel // connection closed message sent
	uplink.AssertExpectations(testing)
}

func TestOutboundConnectionWithError(testing *testing.T) {
	// GIVEN
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	eventChannel := make(chan AdapterEvent, 10)

	urlRemote, _ := url.Parse("tcp://localhost:" + fmt.Sprint(port))
	options := ConnectionAdapterOptions{
		ConnectionId:        "test-connection-id",
		LocalDeviceId:       uuid.New(),
		PeerDeviceId:        uuid.New(),
		PeerDevicePublicKey: "test-peer-device-public-key",
		ResponseInterval:    1000 * time.Millisecond,
		BridgeOptions: messages.BridgeOptions{
			URLRemote: *urlRemote,
		},
	}

	// mock uplink
	uplink := MockUplink{}

	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	assert.Nil(testing, err)

	underTest := NewConnectingOutboundState(options, eventChannel, &uplink, conn)
	err = underTest.Start()
	assert.Nil(testing, err)

	encoderDecoder := encoder.NewEncoderDecoder()
	connectionFailedMessagePayload, _ := encoderDecoder.EncodeConnectionFailedMessage(messages.ConnectionFailedMessage{
		Reason: "connection refused",
	})

	// WHEN
	_, _ = underTest.HandleMessage(messages.Message{
		Header: messages.MessageHeader{
			Type: messages.CF,
		},
		Message: connectionFailedMessagePayload,
	})

	event := <-eventChannel

	// THEN
	assert.Nil(testing, err)
	assert.Contains(testing, event.Message, "connection refused")
	uplink.AssertExpectations(testing)
}

func TestOutboundConnectionStop(testing *testing.T) {
	// GIVEN
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	// Signals
	closeChannel := make(chan bool, 1)
	eventChannel := make(chan AdapterEvent, 10)

	urlRemote, _ := url.Parse("tcp://localhost:" + fmt.Sprint(port))
	options := ConnectionAdapterOptions{
		ConnectionId:        "test-connection-id",
		LocalDeviceId:       uuid.New(),
		PeerDeviceId:        uuid.New(),
		PeerDevicePublicKey: "test-peer-device-public-key",
		ResponseInterval:    1000 * time.Millisecond,
		BridgeOptions: messages.BridgeOptions{
			URLRemote: *urlRemote,
		},
	}

	// mock uplink
	uplink := MockUplink{}

	uplink.On("Send", mock.MatchedBy(func(msg messages.Message) bool {
		if msg.Header.Type == messages.CC {
			closeChannel <- true
		}
		return true
	})).Return(nil)

	underTest := NewConnectingInboundState(options, eventChannel, &uplink)
	err := underTest.Start()
	assert.Nil(testing, err)

	// WHEN
	err = underTest.Close()

	// THEN
	<-closeChannel // connection closed message sent
	assert.Nil(testing, err)
}
