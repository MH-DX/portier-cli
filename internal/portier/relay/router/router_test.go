package router

import (
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/portier/relay/adapter"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRouting(testing *testing.T) {
	// GIVEN
	connectionId := messages.ConnectionId("test-connection-id")
	connectorMock := &ConnectorMock{}
	connectionAdapterMock := &ConnectionAdapterMock{}
	msg := make(chan messages.Message, 10)
	underTest := NewRouter(connectorMock, msg)
	underTest.AddConnection(connectionId, connectionAdapterMock)
	connectionAdapterMock.On("Send", mock.MatchedBy(func(msg messages.Message) bool {
		return msg.Header.CID == connectionId
	})).Return(nil)
	err := underTest.Start()
	if err != nil {
		testing.Errorf("expected err to be nil, got %v", err)
	}

	// WHEN
	err = underTest.HandleMessage(messages.Message{
		Header: messages.MessageHeader{
			From: uuid.New(),
			To:   uuid.New(),
			Type: messages.D,
			CID:  connectionId,
		},
		Message: []byte("Hello, world!"),
	})

	// THEN
	assert.Nil(testing, err)
	connectionAdapterMock.AssertExpectations(testing)
}

func TestConnectionOpen(testing *testing.T) {
	// GIVEN
	connectionId := messages.ConnectionId("test-connection-id")
	connectorMock := &ConnectorMock{}
	msg := make(chan messages.Message, 10)
	underTest := NewRouter(connectorMock, msg)

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

	connectorMock.On("CreateInboundConnection", mock.MatchedBy(func(h messages.MessageHeader) bool {
		return h.Type == messages.CO && h.CID == connectionId
	}), bridgeOptions, pcKey).Return(nil)

	err := underTest.Start()
	if err != nil {
		testing.Errorf("expected err to be nil, got %v", err)
	}

	// WHEN
	err = underTest.HandleMessage(messages.Message{
		Header: messages.MessageHeader{
			From: uuid.New(),
			To:   uuid.New(),
			Type: messages.CO,
			CID:  connectionId,
		},
		Message: connectionOpenMessagePayload,
	})

	// THEN
	assert.Nil(testing, err)
	connectorMock.AssertExpectations(testing)
}

func TestConnectionNotFound(testing *testing.T) {
	// GIVEN
	connectionId := messages.ConnectionId("test-connection-id")
	connectorMock := &ConnectorMock{}
	msg := make(chan messages.Message, 10)
	underTest := NewRouter(connectorMock, msg)

	// WHEN
	err := underTest.HandleMessage(messages.Message{
		Header: messages.MessageHeader{
			From: uuid.New(),
			To:   uuid.New(),
			Type: messages.D,
			CID:  connectionId,
		},
		Message: []byte("Hello, world!"),
	})

	// THEN
	assert.NotNil(testing, err)
	assert.Contains(testing, err.Error(), "connection does not exist")
	connectorMock.AssertExpectations(testing)
	assert.True(testing, false, "implement sending a connection not found message")
}

// ConnectorMock is a mock for the connector
type ConnectorMock struct {
	mock.Mock
}

func (c *ConnectorMock) CreateInboundConnection(header messages.MessageHeader, options messages.BridgeOptions, pcKey string) error {
	args := c.Called(header, options, pcKey)
	return args.Error(0)
}

type ConnectionAdapterMock struct {
	mock.Mock
}

func (c *ConnectionAdapterMock) Start() (chan adapter.AdapterEvent, error) {
	args := c.Called()
	return args.Get(0).(chan adapter.AdapterEvent), args.Error(1)
}

func (c *ConnectionAdapterMock) Stop() error {
	args := c.Called()
	return args.Error(0)
}

func (c *ConnectionAdapterMock) Send(msg messages.Message) error {
	args := c.Called(msg)
	return args.Error(0)
}
