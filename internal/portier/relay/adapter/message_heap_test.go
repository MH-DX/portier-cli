package adapter

import (
	"testing"

	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
)

func TestInsertAndReturn(testing *testing.T) {
	// GIVEN
	underTest := NewMessageHeap(MessageHeapOptions{
		MaxQueueSize: 1,
		MaxQueueGap:  0,
	})

	// WHEN
	result, err := underTest.Test(messages.DataMessage{
		Seq:  uint64(0),
		Data: []byte("test"),
	})

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
	if len(result) != 1 {
		testing.Errorf("Unexpected result: %v", result)
	}
	if result[0].Seq != uint64(0) {
		testing.Errorf("Unexpected result: %v", result)
	}
	if string(result[0].Data) != "test" {
		testing.Errorf("Unexpected result: %v", result)
	}
}

func TestInsertAndReturn2(testing *testing.T) {
	// GIVEN
	underTest := NewMessageHeap(MessageHeapOptions{
		MaxQueueSize: 3,
		MaxQueueGap:  2,
	})
	_, _ = underTest.Test(messages.DataMessage{
		Seq: uint64(1),
	})
	_, _ = underTest.Test(messages.DataMessage{
		Seq: uint64(2),
	})

	// WHEN
	result, err := underTest.Test(messages.DataMessage{
		Seq: uint64(0),
	})

	// THEN
	if err != nil {
		testing.Errorf("Unexpected error: %v", err)
	}
	if len(result) != 3 {
		testing.Errorf("Unexpected result: %v", result)
	}
	if result[0].Seq != uint64(0) {
		testing.Errorf("Unexpected result: %v", result)
	}
	if result[1].Seq != uint64(1) {
		testing.Errorf("Unexpected result: %v", result)
	}
	if result[2].Seq != uint64(2) {
		testing.Errorf("Unexpected result: %v", result)
	}
}

func TestInsertMaxGapExceeded(testing *testing.T) {
	// GIVEN
	underTest := NewMessageHeap(MessageHeapOptions{
		MaxQueueSize: 1,
		MaxQueueGap:  2,
	})

	// WHEN
	_, err := underTest.Test(messages.DataMessage{
		Seq: uint64(3),
	})

	// THEN
	if err == nil {
		testing.Errorf("Expected error")
	}
	if err.Error() != "gap_too_large" {
		testing.Errorf("Unexpected error: %v", err)
	}
}

func TestInsertMaxGapExceeded2(testing *testing.T) {
	// GIVEN
	underTest := NewMessageHeap(MessageHeapOptions{
		MaxQueueSize: 2,
		MaxQueueGap:  2,
	})

	// WHEN
	_, _ = underTest.Test(messages.DataMessage{
		Seq: uint64(1),
	})

	_, err := underTest.Test(messages.DataMessage{
		Seq: uint64(4),
	})

	// THEN
	if err == nil {
		testing.Errorf("Expected error")
	}
	if err.Error() != "gap_too_large" {
		testing.Errorf("Unexpected error: %v", err)
	}
}

func TestInsertOld(testing *testing.T) {
	// GIVEN
	underTest := NewMessageHeap(MessageHeapOptions{
		MaxQueueSize: 1,
		MaxQueueGap:  0,
	})

	// WHEN
	_, _ = underTest.Test(messages.DataMessage{
		Seq: uint64(0),
	})

	_, err := underTest.Test(messages.DataMessage{
		Seq: uint64(0),
	})

	// THEN
	if err == nil {
		testing.Errorf("Expected error")
	}
	if err.Error() != "old_message" {
		testing.Errorf("Unexpected error: %v", err)
	}
}

func TestInsertOld2(testing *testing.T) {
	// GIVEN
	underTest := NewMessageHeap(MessageHeapOptions{
		MaxQueueSize: 2,
		MaxQueueGap:  1,
	})

	// WHEN
	_, _ = underTest.Test(messages.DataMessage{
		Seq: uint64(0),
	})
	_, _ = underTest.Test(messages.DataMessage{
		Seq: uint64(1),
	})
	_, err := underTest.Test(messages.DataMessage{
		Seq: uint64(0),
	})

	// THEN
	if err == nil {
		testing.Errorf("Expected error")
	}
	if err.Error() != "old_message" {
		testing.Errorf("Unexpected error: %v", err)
	}
}
