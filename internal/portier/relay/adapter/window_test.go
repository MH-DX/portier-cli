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
	return WindowOptions{
		InitialCap: cap,
		InitialRTO: rto,
		RTTFactor:  1.5,
	}
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
	_, _, err := underTest.ack(uint64(0), false)

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
	rtt, retrans, err := underTest.ack(uint64(0), false)

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
	if rtt == 0 {
		testing.Errorf("Expected rtt")
	}
	if retrans {
		testing.Errorf("Unexpected retrans")
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
	rtt, retrans, err := underTest.ack(uint64(0), false)

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
	if rtt == 0 {
		testing.Errorf("Expected rtt")
	}
	if retrans {
		testing.Errorf("Unexpected retrans")
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
	_, retrans, err := underTest.ack(uint64(1), false)

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
	if retrans {
		testing.Errorf("Unexpected retrans")
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
	_, retrans, err := underTest.ack(uint64(0), false)

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
	if retrans {
		testing.Errorf("Unexpected retrans")
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
	underTest.ack(uint64(0), true) // should cause retransmission flag for 1 and 2 as well

	// WHEN
	_, retrans1, err1 := underTest.ack(uint64(1), false)
	_, retrans2, err2 := underTest.ack(uint64(2), false)

	// THEN
	if err1 != nil {
		testing.Errorf("Unexpected error: %v", err1)
	}
	if !retrans1 {
		testing.Errorf("Expected retrans")
	}
	if err2 != nil {
		testing.Errorf("Unexpected error: %v", err2)
	}
	if !retrans2 {
		testing.Errorf("Expected retrans")
	}
}
