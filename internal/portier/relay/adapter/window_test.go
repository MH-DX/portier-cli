package adapter

import (
	"context"
	"testing"
	"time"

	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	windowitem "github.com/marinator86/portier-cli/internal/portier/relay/window_item"
	"github.com/stretchr/testify/mock"
)

func createMessage(seq uint64, length int) messages.Message {
	return messages.Message{
		Message: make([]byte, length),
	}
}

func createOptions(cap int) WindowOptions {
	return createOptions2(cap, 300000000)
}

func createOptions2(cap int, rto float64) WindowOptions {
	options := NewDefaultWindowOptions()
	options.InitialCap = float64(cap)
	options.InitialRTO = rto
	return options
}

func TestWindowInsert(testing *testing.T) {
	// GIVEN
	mockUplink := MockUplink{}
	mockUplink.On("Send", mock.Anything).Return(nil)
	mockRtoHeap := MockRtoHeap{}
	mockRtoHeap.On("Add", mock.Anything).Return(nil)
	options := createOptions(4)
	underTest := newWindow(context.Background(), options, &mockUplink, &mockRtoHeap)
	msg := createMessage(uint64(0), 2)

	// WHEN
	err := underTest.add(msg, 2)

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
	if underTest.(*window).queue.Length() != 1 {
		testing.Errorf("Unexpected queue length: %v", underTest.(*window).queue.Length())
	}
	windowItem := underTest.(*window).queue.Peek().(*windowitem.WindowItem)
	if windowItem.Seq != 2 {
		testing.Errorf("Unexpected seq: %v", windowItem.Seq)
	}
	if windowItem.Time.IsZero() {
		testing.Errorf("Unexpected time: %v", windowItem.Time)
	}
	if windowItem.Rto != windowItem.Time.Add(time.Duration(options.InitialRTO)) {
		testing.Errorf("Unexpected rto: %v", windowItem.Rto)
	}
}

func TestWindowInsertFullBlock(testing *testing.T) {
	// GIVEN
	var mockUplink MockUplink = MockUplink{}
	mockUplink.On("Send", mock.Anything).Return(nil)
	mockRtoHeap := MockRtoHeap{}
	mockRtoHeap.On("Add", mock.Anything).Return(nil)
	underTest := newWindow(context.Background(), createOptions(2), &mockUplink, &mockRtoHeap)
	_ = underTest.add(createMessage(uint64(0), 2), 0)
	calledChan := make(chan bool, 1)
	addedChan := make(chan time.Duration, 1)

	go func() {
		calledChan <- true
		calledTime := time.Now()
		_ = underTest.add(createMessage(uint64(1), 1), 1)
		addedChan <- time.Since(calledTime)
	}()

	// WHEN
	<-calledChan
	time.Sleep(1010 * time.Millisecond)
	err := underTest.ack(uint64(0), false)

	// THEN
	returnTime := <-addedChan
	if returnTime < 1*time.Second {
		testing.Errorf("Expected blocking")
	}
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
	if underTest.(*window).currentSize != 1 {
		testing.Errorf("Unexpected currentSize: %v", underTest.(*window).currentSize)
	}
}

func TestWindowInsertAck(testing *testing.T) {
	// GIVEN
	var mockUplink MockUplink = MockUplink{}
	mockUplink.On("Send", mock.Anything).Return(nil)
	mockRtoHeap := MockRtoHeap{}
	mockRtoHeap.On("Add", mock.Anything).Return(nil)
	underTest := newWindow(context.Background(), createOptions(1), &mockUplink, &mockRtoHeap)
	_ = underTest.add(createMessage(uint64(0), 1), 0)

	// WHEN
	err := underTest.ack(uint64(0), false)

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
}

func TestWindowInsertAck2(testing *testing.T) {
	// GIVEN
	var mockUplink MockUplink = MockUplink{}
	mockUplink.On("Send", mock.Anything).Return(nil)
	mockRtoHeap := MockRtoHeap{}
	mockRtoHeap.On("Add", mock.Anything).Return(nil)
	underTest := newWindow(context.Background(), createOptions(2), &mockUplink, &mockRtoHeap)
	_ = underTest.add(createMessage(uint64(0), 1), 0)
	_ = underTest.add(createMessage(uint64(1), 1), 1)

	// WHEN
	err := underTest.ack(uint64(0), false)

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
	if underTest.(*window).currentSize != 1 {
		testing.Errorf("Unexpected currentSize: %v", underTest.(*window).currentSize)
	}
}

func TestWindowInsertAck3(testing *testing.T) {
	// GIVEN
	var mockUplink MockUplink = MockUplink{}
	mockUplink.On("Send", mock.Anything).Return(nil)
	mockRtoHeap := MockRtoHeap{}
	mockRtoHeap.On("Add", mock.Anything).Return(nil)
	underTest := newWindow(context.Background(), createOptions(3), &mockUplink, &mockRtoHeap)
	_ = underTest.add(createMessage(uint64(0), 1), 0)
	_ = underTest.add(createMessage(uint64(1), 1), 1)
	_ = underTest.add(createMessage(uint64(2), 1), 2)

	// WHEN
	err := underTest.ack(uint64(1), false)

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
	if underTest.(*window).currentSize != 3 {
		testing.Errorf("Unexpected currentSize: %v", underTest.(*window).currentSize)
	}
}

func TestWindowInsertAck4(testing *testing.T) {
	// GIVEN
	var mockUplink MockUplink = MockUplink{}
	mockUplink.On("Send", mock.Anything).Return(nil)
	mockRtoHeap := MockRtoHeap{}
	mockRtoHeap.On("Add", mock.Anything).Return(nil)
	underTest := newWindow(context.Background(), createOptions(3), &mockUplink, &mockRtoHeap)
	_ = underTest.add(createMessage(uint64(0), 1), 0)
	_ = underTest.add(createMessage(uint64(1), 1), 1)
	_ = underTest.add(createMessage(uint64(2), 1), 2)
	_ = underTest.ack(uint64(1), false)

	// WHEN
	err := underTest.ack(uint64(0), false)

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
	if underTest.(*window).currentSize != 1 {
		testing.Errorf("Unexpected currentSize: %v", underTest.(*window).currentSize)
	}
}

func TestWindowInsertAckRetransmission(testing *testing.T) {
	// GIVEN
	var mockUplink MockUplink = MockUplink{}
	mockUplink.On("Send", mock.Anything).Return(nil)
	mockRtoHeap := MockRtoHeap{}
	mockRtoHeap.On("Add", mock.Anything).Return(nil)
	underTest := newWindow(context.Background(), createOptions(3), &mockUplink, &mockRtoHeap)
	_ = underTest.add(createMessage(uint64(0), 1), 0)
	_ = underTest.add(createMessage(uint64(1), 1), 1)
	_ = underTest.add(createMessage(uint64(2), 1), 2)

	// WHEN
	err := underTest.ack(uint64(1), true) // should cause retransmission flag for 1 and 2 as well

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
	if underTest.(*window).currentSize != 3 {
		testing.Errorf("Unexpected currentSize: %v", underTest.(*window).currentSize)
	}
	windowItem0 := underTest.(*window).queue.Get(0).(*windowitem.WindowItem)
	if windowItem0.Retransmitted {
		testing.Errorf("Unexpected retransmitted: %v", windowItem0.Retransmitted)
	}
	windowItem1 := underTest.(*window).queue.Get(1).(*windowitem.WindowItem)
	if !windowItem1.Retransmitted {
		testing.Errorf("Unexpected retransmitted: %v", windowItem1.Retransmitted)
	}
	windowItem2 := underTest.(*window).queue.Get(2).(*windowitem.WindowItem)
	if !windowItem2.Retransmitted {
		testing.Errorf("Unexpected retransmitted: %v", windowItem2.Retransmitted)
	}
}

func TestWindowInsertRtt(testing *testing.T) {
	// GIVEN
	var mockUplink MockUplink = MockUplink{}
	mockUplink.On("Send", mock.Anything).Return(nil)
	mockRtoHeap := MockRtoHeap{}
	mockRtoHeap.On("Add", mock.Anything).Return(nil)
	underTest := newWindow(context.Background(), createOptions(3), &mockUplink, &mockRtoHeap)
	_ = underTest.add(createMessage(uint64(0), 1), 0)
	_ = underTest.add(createMessage(uint64(1), 1), 1)
	_ = underTest.add(createMessage(uint64(2), 1), 2)

	// WHEN
	time.Sleep(100 * time.Millisecond)
	_ = underTest.ack(uint64(0), false)
	_ = underTest.ack(uint64(1), false)
	_ = underTest.ack(uint64(2), false)

	// THEN
	if underTest.(*window).stats.SRTT < 100000000 {
		testing.Errorf("Unexpected SRTT: %v", underTest.(*window).stats.SRTT)
	}
	if underTest.(*window).stats.SRTT > 150000000 {
		testing.Errorf("Unexpected SRTT: %v", underTest.(*window).stats.SRTT)
	}
}

type MockRtoHeap struct {
	mock.Mock
}

func (m *MockRtoHeap) Add(item *windowitem.WindowItem) error {
	args := m.Called(item)
	return args.Error(0)
}
