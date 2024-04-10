package rto_heap

import (
	"context"
	"testing"
	"time"

	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
	windowitem "github.com/marinator86/portier-cli/internal/portier/relay/window_item"
	"github.com/stretchr/testify/mock"
)

func TestInsertAndAck(testing *testing.T) {
	// GIVEN
	header := messages.MessageHeader{}
	encryptedData := []byte("encrypted")
	expectedMessage := messages.Message{
		Header:  header,
		Message: encryptedData,
	}
	mockEncryption := new(MockEncryption)
	mockEncryption.On("Encrypt", mock.Anything, mock.Anything).Return(encryptedData, nil)
	encoderDecoder := new(encoder.MockEncoderDecoder)
	encoderDecoder.On("EncodeDataMessage", mock.Anything).Return([]byte("dataMsg"), nil)
	encoderDecoder.On("DecodeDataMessage", mock.Anything).Return(messages.DataMessage{}, nil)
	options := RtoHeapOptions{
		MaxQueueSize: 1,
	}
	mockUplink := new(MockUplink)
	mockUplink.On("Send", expectedMessage).Return(nil)
	underTest := NewRtoHeap(context.Background(), options, mockUplink, encoderDecoder, mockEncryption)
	item := &windowitem.WindowItem{
		Msg: messages.Message{
			Header: header,
		},
		Seq:         0,
		RtoDuration: time.Millisecond * 100,
	}

	// WHEN
	err := underTest.Add(item)

	go func() {
		time.Sleep(time.Millisecond * 120)
		item.Acked = true
	}()

	for len(underTest.(*rtoHeap).queue) > 0 {
		time.Sleep(time.Millisecond * 10)
	}

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
	mockUplink.AssertNumberOfCalls(testing, "Send", 1)
	mockUplink.AssertExpectations(testing)
	mockEncryption.AssertNumberOfCalls(testing, "Encrypt", 1)
	mockEncryption.AssertExpectations(testing)
	encoderDecoder.AssertNumberOfCalls(testing, "EncodeDataMessage", 1)
	encoderDecoder.AssertNumberOfCalls(testing, "DecodeDataMessage", 1)
	encoderDecoder.AssertExpectations(testing)
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

type MockEncryption struct {
	mock.Mock
}

func (m *MockEncryption) Decrypt(header messages.MessageHeader, data []byte) ([]byte, error) {
	args := m.Called(header, data)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockEncryption) Encrypt(header messages.MessageHeader, data []byte) ([]byte, error) {
	args := m.Called(header, data)
	return args.Get(0).([]byte), args.Error(1)
}
