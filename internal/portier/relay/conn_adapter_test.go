package relay

import (
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/stretchr/testify/mock"
)

func TestConnection(testing *testing.T) {
	// GIVEN
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	// Signals
	connectionChannel := make(chan bool)
	acceptedChannel := make(chan bool, 100)
	closedChannel := make(chan bool, 100)

	urlRemote, _ := url.Parse("tcp://localhost:" + fmt.Sprint(port))
	options := ConnectionAdapterOptions{
		ConnectionId:        "test-connection-id",
		LocalDeviceId:       uuid.New(),
		PeerDeviceId:        uuid.New(),
		PeerDevicePublicKey: "test-peer-device-public-key",
		BridgeOptions: messages.BridgeOptions{
			URLRemote: *urlRemote,
		},
	}
	encoderDecoder := encoder.EncoderDecoder{}

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

	underTest := NewConnectingInboundState(options, &encoderDecoder, &uplink, 1000)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			testing.Errorf("expected err to be nil, got %v", err)
		}
		defer conn.Close()
		connectionChannel <- true
	}()

	// WHEN
	underTest.Start()

	// THEN
	<-connectionChannel // tcp connection established
	<-acceptedChannel   // connection accepted message sent
	underTest.Stop()
	<-closedChannel // connection closed message sent
	uplink.AssertExpectations(testing)
}

type MockUplink struct {
	mock.Mock
}

func (m *MockUplink) Connect() (chan []byte, error) {
	m.Called()
	return nil, nil
}

func (m *MockUplink) Send(message messages.Message) error {
	args := m.Called(message)
	return args.Error(0)
}

func (m *MockUplink) Close() error {
	m.Called()
	return nil
}

func (m *MockUplink) Events() <-chan UplinkEvent {
	m.Called()
	return nil
}
