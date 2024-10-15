package rto_heap

import (
	"context"
	"testing"
	"time"

	"github.com/mh-dx/portier-cli/internal/portier/relay/encoder"
	"github.com/mh-dx/portier-cli/internal/portier/relay/messages"
	"github.com/mh-dx/portier-cli/internal/portier/relay/uplink"
	windowitem "github.com/mh-dx/portier-cli/internal/portier/relay/window_item"
	"github.com/stretchr/testify/mock"
)

func TestInsertAndAck(testing *testing.T) {
	// GIVEN
	rtoDuration := time.Millisecond * 500
	header := messages.MessageHeader{}
	data := []byte("dataMsg")
	expectedMessage := messages.Message{
		Header:  header,
		Message: data,
	}
	encoderDecoder := new(encoder.MockEncoderDecoder)
	encoderDecoder.On("EncodeDataMessage", mock.Anything).Return([]byte("dataMsg"), nil)
	encoderDecoder.On("DecodeDataMessage", mock.Anything).Return(messages.DataMessage{}, nil)
	options := RtoHeapOptions{
		MaxQueueSize: 1,
	}
	mockUplink := new(MockUplink)
	mockUplink.On("Send", expectedMessage).Return(nil)
	underTest := NewRtoHeap(context.Background(), options, mockUplink, encoderDecoder)
	item := &windowitem.WindowItem{
		Msg: messages.Message{
			Header: header,
		},
		Seq:         0,
		RtoDuration: rtoDuration,
		Rto:         time.Now().Add(rtoDuration),
	}
	added := time.Now()
	resentChannel := make(chan time.Time)

	// WHEN
	// measure the time it takes to insert an item and ack it
	err := underTest.Add(item)
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}

	// THEN
	// wait until send is called, then ack the item
	go func() {
		for len(mockUplink.Calls) == 0 {
			time.Sleep(time.Millisecond * 50)
		}
		if mockUplink.Calls[0].Method != "Send" {
			panic("Send not called")
		}
		resentChannel <- time.Now()
	}()

	resent := <-resentChannel
	item.Acked = true

	// wait until the item is removed from the queue
	for len(underTest.(*rtoHeap).queue) > 0 {
		time.Sleep(time.Millisecond * 50)
	}

	timeTillResent := resent.Sub(added)

	if timeTillResent < rtoDuration {
		testing.Errorf("Item was resent too early: %v", resent.Sub(added))
	}

	if timeTillResent > rtoDuration*2 {
		testing.Errorf("Item was resent too late: %v", resent.Sub(added))
	}

	mockUplink.AssertNumberOfCalls(testing, "Send", 1)
	mockUplink.AssertExpectations(testing)
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
