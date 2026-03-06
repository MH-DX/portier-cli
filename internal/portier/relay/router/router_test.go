package router

import (
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mh-dx/portier-cli/internal/portier/relay/adapter"
	"github.com/mh-dx/portier-cli/internal/portier/relay/encoder"
	"github.com/mh-dx/portier-cli/internal/portier/relay/messages"
	"github.com/mh-dx/portier-cli/internal/portier/relay/uplink"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRouting(testing *testing.T) {
	// GIVEN
	connectionId := messages.ConnectionID("test-connection-id")
	connectionAdapterMock := &ConnectionAdapterMock{}
	msg := make(chan messages.Message, 10)
	events := make(chan adapter.AdapterEvent, 10)
	uplinkMock := &MockUplink{}
	ptls := &MockPTLS{}
	underTest := NewRouter(uplinkMock, msg, events, ptls, nil)
	underTest.AddConnection(connectionId, connectionAdapterMock)
	connectionAdapterMock.On("Send", mock.MatchedBy(func(msg messages.Message) bool {
		return msg.Header.CID == connectionId
	})).Return(nil)
	err := underTest.Start()
	assert.Nil(testing, err)

	// WHEN
	underTest.HandleMessage(messages.Message{
		Header: messages.MessageHeader{
			From: uuid.New(),
			To:   uuid.New(),
			Type: messages.D,
			CID:  connectionId,
		},
		Message: []byte("Hello, world!"),
	})

	// THEN
	connectionAdapterMock.AssertExpectations(testing)
}

func TestConnectionOpen(testing *testing.T) {
	// GIVEN
	forwarded, _ := net.Listen("tcp", "127.0.0.1:0")
	defer forwarded.Close()
	connectionId := messages.ConnectionID("test-connection-id")
	msg := make(chan messages.Message, 10)
	events := make(chan adapter.AdapterEvent, 10)
	uplinkMock := &MockUplink{}
	uplinkMock.On("Send", mock.MatchedBy(func(msg messages.Message) bool {
		return true
	})).Return(nil)
	ptls := &MockPTLS{}
	ptls.On("TestEndpointURL", mock.Anything).Return(false)

	underTest := NewRouter(uplinkMock, msg, events, ptls, nil)

	remoteUrl, _ := url.Parse("tcp://" + forwarded.Addr().String())
	bridgeOptions := messages.BridgeOptions{
		URLRemote: *remoteUrl,
	}
	connectionOpenMessage := messages.ConnectionOpenMessage{
		BridgeOptions: bridgeOptions,
	}
	encoderDecoder := encoder.NewEncoderDecoder()
	connectionOpenMessagePayload, _ := encoderDecoder.EncodeConnectionOpenMessage(connectionOpenMessage)

	err := underTest.Start()
	assert.Nil(testing, err)

	// WHEN
	underTest.HandleMessage(messages.Message{
		Header: messages.MessageHeader{
			From: uuid.New(),
			To:   uuid.New(),
			Type: messages.CO,
			CID:  connectionId,
		},
		Message: connectionOpenMessagePayload,
	})

	// THEN
	assert.NotNil(testing, underTest.(*router).connections[connectionId])
}

func TestConnectionNotFound(testing *testing.T) {
	// GIVEN
	connectionId := messages.ConnectionID("test-connection-id")
	msg := make(chan messages.Message, 10)
	events := make(chan adapter.AdapterEvent, 10)
	uplinkMock := &MockUplink{}
	uplinkMock.On("Send", mock.MatchedBy(func(msg messages.Message) bool {
		return msg.Header.Type == messages.NF
	})).Return(nil)
	ptls := &MockPTLS{}
	underTest := NewRouter(uplinkMock, msg, events, ptls, nil)

	// WHEN
	underTest.HandleMessage(messages.Message{
		Header: messages.MessageHeader{
			From: uuid.New(),
			To:   uuid.New(),
			Type: messages.D,
			CID:  connectionId,
		},
		Message: []byte("Hello, world!"),
	})

	// THEN
	uplinkMock.AssertExpectations(testing)
}

func TestConnectionOpenReportsInitiationFailure(testing *testing.T) {
	msg := make(chan messages.Message, 10)
	events := make(chan adapter.AdapterEvent, 10)
	uplinkMock := &MockUplink{}
	uplinkMock.On("Send", mock.Anything).Return(nil)
	ptls := &MockPTLS{}
	ptls.On("TestEndpointURL", mock.Anything).Return(false)

	reportCh := make(chan InitiationFailureReport, 1)
	underTest := NewRouter(uplinkMock, msg, events, ptls, func(report InitiationFailureReport) {
		reportCh <- report
	})

	remoteURL, _ := url.Parse("tcp://127.0.0.1:1")
	connectionOpenMessage := messages.ConnectionOpenMessage{
		BridgeOptions: messages.BridgeOptions{URLRemote: *remoteURL},
	}
	encoderDecoder := encoder.NewEncoderDecoder()
	connectionOpenMessagePayload, _ := encoderDecoder.EncodeConnectionOpenMessage(connectionOpenMessage)

	underTest.HandleMessage(messages.Message{
		Header: messages.MessageHeader{
			From: uuid.New(),
			To:   uuid.New(),
			Type: messages.CO,
			CID:  messages.ConnectionID("cid-report-test"),
		},
		Message: connectionOpenMessagePayload,
	})

	select {
	case report := <-reportCh:
		assert.Equal(testing, "TARGET_INITIATION_ERROR", report.ErrorCode)
		assert.Equal(testing, "cid-report-test", report.ConnectionID)
		assert.Equal(testing, "tcp://127.0.0.1:1", report.RemoteURL)
	case <-time.After(2 * time.Second):
		testing.Fatal("expected initiation failure report to be emitted")
	}
}

type ConnectionAdapterMock struct {
	mock.Mock
}

func (c *ConnectionAdapterMock) Start() error {
	args := c.Called()
	return args.Error(0)
}

func (c *ConnectionAdapterMock) Close() error {
	args := c.Called()
	return args.Error(0)
}

func (c *ConnectionAdapterMock) Send(msg messages.Message) {
	c.Called(msg)
}

type MockUplink struct {
	mock.Mock
}

func (m *MockUplink) Connect() (<-chan messages.Message, error) {
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

func (m *MockUplink) Events() <-chan uplink.Event {
	m.Called()
	return nil
}

type MockPTLS struct {
	mock.Mock
}

func (m *MockPTLS) TestEndpointURL(endpoint url.URL) bool {
	args := m.Called(endpoint)
	return args.Bool(0)
}

func (m *MockPTLS) CreateClientAndBridge(conn net.Conn, peerDeviceID uuid.UUID) (net.Conn, func() error, error) {
	args := m.Called(conn, peerDeviceID)
	return args.Get(0).(net.Conn), args.Get(1).(func() error), args.Error(2)
}

func (m *MockPTLS) CreateServerAndBridge(conn net.Conn, peerDeviceID uuid.UUID) (net.Conn, error) {
	args := m.Called(conn, peerDeviceID)
	return args.Get(0).(net.Conn), args.Error(1)
}
