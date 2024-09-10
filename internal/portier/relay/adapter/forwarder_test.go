package adapter

import (
	"fmt"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestForwardingToConnectionServer(testing *testing.T) {
	// GIVEN
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	assert.Nil(testing, err)
	s_conn, _ := listener.Accept()
	defer s_conn.Close()

	localDeviceId := uuid.New()
	peerDeviceId := uuid.New()

	eventChannel := make(chan AdapterEvent, 10)

	options := ForwarderOptions{
		Throughput:    1000,
		LocalDeviceID: localDeviceId,
		PeerDeviceID:  peerDeviceId,
		ConnectionID:  "test-connection-id",
	}

	// mock uplink
	uplink := MockUplink{}
	uplink.On("Send", mock.Anything).Return(nil)

	underTest := NewForwarder(options, conn, &uplink, eventChannel)

	err = underTest.Start()
	assert.Nil(testing, err)

	dm := messages.DataMessage{
		Seq:  0,
		Data: []byte("test"),
	}

	dmEncoded, _ := encoder.NewEncoderDecoder().EncodeDataMessage(dm)

	// WHEN
	// send a message to the send channel and check if it is received by the conn

	_ = underTest.SendAsync(messages.Message{
		Header: messages.MessageHeader{
			From: peerDeviceId,
			To:   localDeviceId,
			Type: messages.D,
			CID:  "test-connection-id",
		},
		Message: dmEncoded,
	})

	// THEN
	buf := make([]byte, 1024)
	n, err := s_conn.Read(buf)
	assert.Nil(testing, err)
	assert.Equal(testing, []byte("test"), buf[:n])

	underTest.Close()
	uplink.AssertExpectations(testing)
}

func TestForwardingToUplink(testing *testing.T) {
	// GIVEN
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	assert.Nil(testing, err)
	s_conn, _ := listener.Accept()
	defer s_conn.Close()

	localDeviceId := uuid.New()
	peerDeviceId := uuid.New()

	// Signals
	msgChannel := make(chan messages.Message, 1)
	eventChannel := make(chan AdapterEvent, 10)

	options := ForwarderOptions{
		Throughput:     1000,
		LocalDeviceID:  localDeviceId,
		PeerDeviceID:   peerDeviceId,
		ConnectionID:   "test-connection-id",
		ReadTimeout:    100,
		ReadBufferSize: 1024,
	}

	// mock uplink
	uplink := MockUplink{}

	uplink.On("Send", mock.MatchedBy(func(msg messages.Message) bool {
		if msg.Header.Type == messages.D {
			msgChannel <- msg
		}
		return true
	})).Return(nil)

	underTest := NewForwarder(options, conn, &uplink, eventChannel)

	err = underTest.Start()
	assert.Nil(testing, err)

	// WHEN
	_, err = s_conn.Write([]byte("test1"))
	assert.Nil(testing, err)
	received1 := <-msgChannel
	_, err = s_conn.Write([]byte("test2"))
	assert.Nil(testing, err)
	received2 := <-msgChannel

	decoder := encoder.NewEncoderDecoder()
	decoded1, _ := decoder.DecodeDataMessage(received1.Message)
	decoded2, _ := decoder.DecodeDataMessage(received2.Message)

	// THEN
	assert.Equal(testing, []byte("test1"), decoded1.Data)
	assert.Equal(testing, []byte("test2"), decoded2.Data)

	underTest.Close()
	uplink.AssertExpectations(testing)
}
