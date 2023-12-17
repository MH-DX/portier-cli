package adapter

import (
	"container/heap"
	"errors"

	messages "github.com/marinator86/portier-cli/internal/portier/relay/messages"
)

type MessageHeapOptions struct {
	// MaxQueueSize is the maximum number of messages that can be queued
	MaxQueueSize int

	// MaxQueueGap is the maximum number of messages that can be missing from the queue
	MaxQueueGap int
}

type MessageHeap interface {

	// Test checks if the sequence number of msg is the next expected sequence number n_seq.
	// If it is, it returns a sequence of messages that are ready to be sent to the
	// port. The first message in the sequence is msg.
	//
	// The sequence is guaranteed to be in order, and contains no gaps, i.e. the
	// sequence number of the last message in the sequence is n_seq + len(sequence) - 1.
	//
	// If n_seq is not the next expected sequence number, it is kept in the queue until
	// a message with the next expected sequence number is received and a sequence
	// containing that message is returned.
	//
	// If the sequence number of msg is less than n_seq, it is considered an old message
	// and will be discarded.
	//
	// If the queue is full, or if the gap between n_seq and the sequence number of msg is
	// larger than MaxQueueGap, it returns an error.
	Test(msg messages.DataMessage) ([]messages.DataMessage, error)
}

// An Item is something we manage in a priority queue.
type Item struct {
	value    messages.DataMessage // The value of the item; arbitrary.
	priority uint64               // The priority of the item in the queue.
	index    int                  // The index of the item in the heap.
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*Item

// internally, the message heap is implemented as a priority queue using a heap from the standard library
type messageHeap struct {
	options MessageHeapOptions
	nSeq    uint64
	queue   PriorityQueue
}

func NewMessageHeap(options MessageHeapOptions) MessageHeap {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)
	return &messageHeap{
		options: options,
		queue:   pq,
		nSeq:    0,
	}
}

func (messageHeap *messageHeap) Test(msg messages.DataMessage) ([]messages.DataMessage, error) {
	if msg.Seq == messageHeap.nSeq {
		// the message is the next expected message
		messageHeap.nSeq++
		// keep popping messages from the queue until we find a gap
		sequence := []messages.DataMessage{msg}
		for len(messageHeap.queue) > 0 {
			item := heap.Pop(&messageHeap.queue).(*Item)
			if item.value.Seq == messageHeap.nSeq {
				messageHeap.nSeq++
				sequence = append(sequence, item.value)
			} else {
				heap.Push(&messageHeap.queue, item)
				break
			}
		}
		return sequence, nil
	} else if msg.Seq < messageHeap.nSeq {
		// the message is old
		return nil, errors.New("old_message")
	} else {
		// the message is new
		if len(messageHeap.queue) == messageHeap.options.MaxQueueSize {
			// the queue is full
			return nil, errors.New("queue_full")
		} else if msg.Seq-messageHeap.nSeq > uint64(messageHeap.options.MaxQueueGap) {
			// the gap is too large
			return nil, errors.New("gap_too_large")
		} else {
			// the message is new, but the queue is not full
			item := &Item{
				value:    msg,
				priority: -msg.Seq,
			}
			heap.Push(&messageHeap.queue, item)
			return nil, nil
		}
	}
}

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return pq[i].priority > pq[j].priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}
