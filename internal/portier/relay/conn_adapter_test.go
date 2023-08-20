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
	listener, _ := net.Listen("tcp", ":0")
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

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
	uplink.On("Send", mock.Anything).Return(nil)
	// create a connection adapter
	underTest := NewConnectingInboundState(options, &encoderDecoder, &uplink, 1000)

	// WHEN
	go func() {
		underTest.Start()
	}()

	// THEN
	conn, err := listener.Accept()
	if err != nil {
		testing.Errorf("expected err to be nil, got %v", err)
	}
	defer conn.Close()
}

type MockUplink struct {
	mock.Mock
}

func (m *MockUplink) Connect() (chan []byte, error) {
	m.Called()
	return nil, nil
}

func (m *MockUplink) Send(payload []byte) error {
	m.Called(payload)
	return nil
}

func (m *MockUplink) Close() error {
	m.Called()
	return nil
}

func (m *MockUplink) Events() <-chan UplinkEvent {
	m.Called()
	return nil
}
