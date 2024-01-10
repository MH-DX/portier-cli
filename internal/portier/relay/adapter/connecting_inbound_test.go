package adapter

import (
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestInboundConnection(testing *testing.T) {
	// GIVEN
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	// Signals
	connectionChannel := make(chan bool, 1)
	acceptedChannel := make(chan bool, 1)
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
		if msg.Header.Type == messages.CA {
			acceptedChannel <- true
		}
		if msg.Header.Type == messages.CC {
			closedChannel <- true
		}
		return true
	})).Return(nil)

	underTest := NewConnectingInboundState(options, eventChannel, &uplink)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			testing.Errorf("expected err to be nil, got %v", err)
		}
		defer conn.Close()
		connectionChannel <- true
	}()

	// WHEN
	_ = underTest.Start()

	// THEN
	<-connectionChannel // tcp connection established
	<-acceptedChannel   // connection accepted message sent
	<-acceptedChannel   // resend connection accepted message sent
	_ = underTest.Stop()
	<-closedChannel // connection closed message sent
	uplink.AssertExpectations(testing)
}

func TestInboundConnectionWithError(testing *testing.T) {
	// GIVEN
	port := 51222

	// Signals
	failedChannel := make(chan bool, 1)
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
		if msg.Header.Type == messages.CF {
			// convert msg.Message to string
			msgText := string(msg.Message)
			assert.Contains(testing, msgText, fmt.Sprint(port))
			assert.Contains(testing, msgText, "refused")
			failedChannel <- true
		}
		return true
	})).Return(nil)

	underTest := NewConnectingInboundState(options, eventChannel, &uplink)

	// WHEN
	err := underTest.Start()

	// THEN
	assert.NotNil(testing, err)
	// assert error contains port and connection refused
	assert.Contains(testing, err.Error(), fmt.Sprint(port))
	assert.Contains(testing, err.Error(), "connection refused")
	<-failedChannel // connection failed message sent
	uplink.AssertExpectations(testing)
}

func TestInboundConnectionStop(testing *testing.T) {
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
	err = underTest.Stop()

	// THEN
	<-closeChannel // connection closed message sent
	assert.Nil(testing, err)
}
