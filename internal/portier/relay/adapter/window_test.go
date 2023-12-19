package adapter

import (
	"testing"
	"time"

	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/stretchr/testify/mock"
)

func createMessage(seq uint64, length int) messages.Message {
	return messages.Message{
		Message: make([]byte, length),
	}
}

func createOptions(cap int) WindowOptions {
	return createOptions2(cap, 50*time.Millisecond)
}

func createOptions2(cap int, rto time.Duration) WindowOptions {
	options := NewDefaultWindowOptions()
	options.InitialCap = float64(cap)
	options.InitialRTO = rto
	return options
}

func TestWindowInsert(testing *testing.T) {
	// GIVEN
	var mockUplink MockUplink = MockUplink{}
	mockUplink.On("Send", mock.Anything).Return(nil)
	options := createOptions(4)
	underTest := NewWindow(options, &mockUplink)
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
	windowItem := underTest.(*window).queue.Peek().(*windowItem)
	if windowItem.seq != 2 {
		testing.Errorf("Unexpected seq: %v", windowItem.seq)
	}
	if windowItem.time.IsZero() {
		testing.Errorf("Unexpected time: %v", windowItem.time)
	}
	if windowItem.rto != windowItem.time.Add(options.InitialRTO) {
		testing.Errorf("Unexpected rto: %v", windowItem.rto)
	}
}

func TestWindowInsertFullBlock(testing *testing.T) {
	// GIVEN
	var mockUplink MockUplink = MockUplink{}
	mockUplink.On("Send", mock.Anything).Return(nil)
	underTest := NewWindow(createOptions(2), &mockUplink)
	underTest.add(createMessage(uint64(0), 2), 0)
	calledChan := make(chan bool, 1)
	addedChan := make(chan time.Duration, 1)

	go func() {
		calledChan <- true
		calledTime := time.Now()
		underTest.add(createMessage(uint64(1), 1), 1)
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
	underTest := NewWindow(createOptions(1), &mockUplink)
	underTest.add(createMessage(uint64(0), 1), 0)

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
	underTest := NewWindow(createOptions(2), &mockUplink)
	underTest.add(createMessage(uint64(0), 1), 0)
	underTest.add(createMessage(uint64(1), 1), 1)

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
	underTest := NewWindow(createOptions(3), &mockUplink)
	underTest.add(createMessage(uint64(0), 1), 0)
	underTest.add(createMessage(uint64(1), 1), 1)
	underTest.add(createMessage(uint64(2), 1), 2)

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
	underTest := NewWindow(createOptions(3), &mockUplink)
	underTest.add(createMessage(uint64(0), 1), 0)
	underTest.add(createMessage(uint64(1), 1), 1)
	underTest.add(createMessage(uint64(2), 1), 2)
	underTest.ack(uint64(1), false)

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
	underTest := NewWindow(createOptions(3), &mockUplink)
	underTest.add(createMessage(uint64(0), 1), 0)
	underTest.add(createMessage(uint64(1), 1), 1)
	underTest.add(createMessage(uint64(2), 1), 2)

	// WHEN
	err := underTest.ack(uint64(1), true) // should cause retransmission flag for 1 and 2 as well

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
	if underTest.(*window).currentSize != 3 {
		testing.Errorf("Unexpected currentSize: %v", underTest.(*window).currentSize)
	}
	windowItem0 := underTest.(*window).queue.Get(0).(*windowItem)
	if windowItem0.retransmitted {
		testing.Errorf("Unexpected retransmitted: %v", windowItem0.retransmitted)
	}
	windowItem1 := underTest.(*window).queue.Get(1).(*windowItem)
	if !windowItem1.retransmitted {
		testing.Errorf("Unexpected retransmitted: %v", windowItem1.retransmitted)
	}
	windowItem2 := underTest.(*window).queue.Get(2).(*windowItem)
	if !windowItem2.retransmitted {
		testing.Errorf("Unexpected retransmitted: %v", windowItem2.retransmitted)
	}
}

func TestWindowInsertRtt(testing *testing.T) {
	// GIVEN
	var mockUplink MockUplink = MockUplink{}
	mockUplink.On("Send", mock.Anything).Return(nil)
	underTest := NewWindow(createOptions(3), &mockUplink)
	underTest.add(createMessage(uint64(0), 1), 0)
	underTest.add(createMessage(uint64(1), 1), 1)
	underTest.add(createMessage(uint64(2), 1), 2)

	// WHEN
	time.Sleep(50 * time.Millisecond)
	underTest.ack(uint64(0), false)
	underTest.ack(uint64(1), false)
	underTest.ack(uint64(2), false)

	// THEN
	if underTest.(*window).stats.SRTT < 50000 {
		testing.Errorf("Unexpected SRTT: %v", underTest.(*window).stats.SRTT)
	}
	if underTest.(*window).stats.SRTT > 100000 {
		testing.Errorf("Unexpected SRTT: %v", underTest.(*window).stats.SRTT)
	}
}
