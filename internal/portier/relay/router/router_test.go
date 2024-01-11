package router

import (
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRouting(testing *testing.T) {
	// GIVEN
	connectionId := messages.ConnectionId("test-connection-id")
	connectionAdapterMock := &ConnectionAdapterMock{}
	msg := make(chan messages.Message, 10)
	events := make(chan<- ConnectionOpenEvent, 10)
	uplinkMock := &MockUplink{}
	underTest := NewRouter(uplinkMock, msg, events)
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
	connectionId := messages.ConnectionId("test-connection-id")
	msg := make(chan messages.Message, 10)
	events := make(chan ConnectionOpenEvent, 10)
	uplinkMock := &MockUplink{}
	underTest := NewRouter(uplinkMock, msg, events)

	pcKey := "test-pc-key"
	remoteUrl, _ := url.Parse("tcp://localhost:1234")
	bridgeOptions := messages.BridgeOptions{
		URLRemote: *remoteUrl,
	}
	connectionOpenMessage := messages.ConnectionOpenMessage{
		BridgeOptions: bridgeOptions,
		PCKey:         pcKey,
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
	connectionOpenEvent := <-events
	if connectionOpenEvent.Header.Type != messages.CO {
		testing.Errorf("expected %v, got %v", messages.CO, connectionOpenEvent.Header.Type)
	}
	if messages.MessageType(connectionOpenEvent.Header.CID) != messages.MessageType(connectionId) {
		testing.Errorf("expected %v, got %v", connectionId, connectionOpenEvent.Header.CID)
	}
}

func TestConnectionNotFound(testing *testing.T) {
	// GIVEN
	connectionId := messages.ConnectionId("test-connection-id")
	connectorMock := &ConnectorMock{}
	msg := make(chan messages.Message, 10)
	events := make(chan<- ConnectionOpenEvent, 10)
	uplinkMock := &MockUplink{}
	uplinkMock.On("Send", mock.MatchedBy(func(msg messages.Message) bool {
		return msg.Header.Type == messages.NF
	})).Return(nil)
	underTest := NewRouter(uplinkMock, msg, events)

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
	connectorMock.AssertExpectations(testing)
}

// ConnectorMock is a mock for the connector.
type ConnectorMock struct {
	mock.Mock
}

func (c *ConnectorMock) CreateInboundConnection(header messages.MessageHeader, options messages.BridgeOptions, pcKey string) {
	c.Called(header, options, pcKey)
}

type ConnectionAdapterMock struct {
	mock.Mock
}

func (c *ConnectionAdapterMock) Start() error {
	args := c.Called()
	return args.Error(0)
}

func (c *ConnectionAdapterMock) Stop() error {
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

func (m *MockUplink) Events() <-chan uplink.UplinkEvent {
	m.Called()
	return nil
}
