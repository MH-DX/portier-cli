package adapter

import (
	"testing"
	"time"

	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
)

func createMessage(seq uint64, length int) messages.DataMessage {
	return messages.DataMessage{
		Seq:  seq,
		Data: make([]byte, length),
	}
}

func TestWindowInsert(testing *testing.T) {
	// GIVEN
	underTest := NewWindow(WindowOptions{
		InitialCap: 4,
	})
	msg := createMessage(uint64(0), 2)

	// WHEN
	err := underTest.add(msg)

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
}

func TestWindowInsertFullBlock(testing *testing.T) {
	// GIVEN
	underTest := NewWindow(WindowOptions{
		InitialCap: 2,
	})
	underTest.add(createMessage(uint64(0), 2))
	calledChan := make(chan bool, 1)
	addedChan := make(chan time.Duration, 1)

	go func() {
		calledChan <- true
		calledTime := time.Now()
		underTest.add(createMessage(uint64(1), 1))
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

func TestWindowInsertFullEnlarge(testing *testing.T) {
	// GIVEN
	underTest := NewWindow(WindowOptions{
		InitialCap: 2,
	})

	// WHEN
	err1 := underTest.add(createMessage(uint64(0), 2))
	underTest.setCap(3)
	err2 := underTest.add(createMessage(uint64(1), 1))

	// THEN
	if err1 != nil {
		testing.Errorf("Unexpected error: %v", err1)
	}
	if err2 != nil {
		testing.Errorf("Unexpected error: %v", err2)
	}
}

func TestWindowInsertAck(testing *testing.T) {
	// GIVEN
	underTest := NewWindow(WindowOptions{
		InitialCap: 1,
	})
	underTest.add(createMessage(uint64(0), 1))

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
	underTest := NewWindow(WindowOptions{
		InitialCap: 2,
	})
	underTest.add(createMessage(uint64(0), 1))
	underTest.add(createMessage(uint64(1), 1))

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
	underTest := NewWindow(WindowOptions{
		InitialCap: 3,
	})
	underTest.add(createMessage(uint64(0), 1))
	underTest.add(createMessage(uint64(1), 1))
	underTest.add(createMessage(uint64(2), 1))

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
	underTest := NewWindow(WindowOptions{
		InitialCap: 3,
	})
	underTest.add(createMessage(uint64(0), 1))
	underTest.add(createMessage(uint64(1), 1))
	underTest.add(createMessage(uint64(2), 1))
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
	underTest := NewWindow(WindowOptions{
		InitialCap: 3,
	})
	underTest.add(createMessage(uint64(0), 1))
	underTest.add(createMessage(uint64(1), 1))
	underTest.add(createMessage(uint64(2), 1))
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
