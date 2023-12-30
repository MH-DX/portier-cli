package rto_heap

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/encryption"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
	windowitem "github.com/marinator86/portier-cli/internal/portier/relay/window_item"
)

type RtoHeapOptions struct {
	// MaxQueueSize is the maximum number of items that can be queued
	MaxQueueSize int
}

type RtoHeap interface {
	Add(item *windowitem.WindowItem) error
}

type item struct {
	value *windowitem.WindowItem // The value of the item; arbitrary.
	index int                    // The index of the item in the heap.
}

// A PriorityQueue implements heap.Interface and holds Items.
type priorityQueue []*item

type rtoHeap struct {
	uplink        uplink.Uplink
	encoder       encoder.EncoderDecoder
	encryption    encryption.Encryption
	options       RtoHeapOptions
	queue         priorityQueue
	timer         *time.Timer
	updateChannel chan bool
	ctx           context.Context
	lock          sync.Mutex
	currentItem   *item
}

func NewDefaultRtoHeapOptions() RtoHeapOptions {
	return RtoHeapOptions{
		MaxQueueSize: 1000000,
	}
}

func NewRtoHeap(ctx context.Context, options RtoHeapOptions, uplink uplink.Uplink, encoder encoder.EncoderDecoder, encryption encryption.Encryption) RtoHeap {
	pq := make(priorityQueue, 0)
	heap.Init(&pq)

	rtoHeap := &rtoHeap{
		uplink:        uplink,
		encoder:       encoder,
		encryption:    encryption,
		options:       options,
		queue:         pq,
		timer:         time.NewTimer(time.Hour),
		updateChannel: make(chan bool, 1),
		ctx:           ctx,
		lock:          sync.Mutex{},
	}

	go rtoHeap.process()

	return rtoHeap
}

func (r *rtoHeap) Add(newItem *windowitem.WindowItem) error {
	if len(r.queue) >= r.options.MaxQueueSize {
		return errors.New("queue is full")
	}

	wrapper := &item{
		value: newItem,
	}
	r.lock.Lock()
	heap.Push(&r.queue, wrapper)
	r.updateTimer()
	r.lock.Unlock()
	return nil
}

func (r *rtoHeap) updateTimer() {
	if len(r.queue) != 0 {
		if !r.timer.Stop() {
			select {
			case <-r.timer.C:
			default:
			}
		}

		// pop item from queue as long as it is acked
		for len(r.queue) > 0 && r.queue[0].value.Acked {
			//fmt.Printf("PRE: Removing message with seq %d\n", r.queue[0].value.Seq)
			heap.Pop(&r.queue)
		}

		if len(r.queue) == 0 {
			return
		}

		nextRTO := r.queue[0].value.RtoDuration
		r.timer.Reset(nextRTO)
		r.currentItem = r.queue[0]
	}
}

func (r *rtoHeap) process() {

	for {
		r.lock.Lock()
		r.updateTimer()
		r.lock.Unlock()

		select {
		case <-r.timer.C:
			// Timer expired, check the item and resend if necessary
			r.lock.Lock()
			item := r.currentItem
			if item.value.Acked {
				//fmt.Printf("Removing message with seq %d\n", item.value.Seq)
				heap.Remove(&r.queue, item.index)
			} else {
				//fmt.Printf("Retransmitting message with seq %d\n", item.value.Seq)
				item.value.Rto = time.Now().Add(item.value.RtoDuration)
				item.value.Retransmitted = true
				heap.Fix(&r.queue, item.index)

				// decode the datamessage and update the retransmitted flag
				dataMsg, err := r.encoder.DecodeDataMessage(item.value.Msg.Message)
				if err != nil {
					fmt.Println("Error decoding data message")
					continue
				}
				dataMsg.Re = true
				// encode the datamessage
				dmBytes, err := r.encoder.EncodeDataMessage(dataMsg)
				if err != nil {
					fmt.Println("Error encoding data message")
					continue
				}
				// encrypt the data
				encrypted, err := r.encryption.Encrypt(item.value.Msg.Header, dmBytes)
				if err != nil {
					fmt.Printf("error encrypting data: %s\n", err)
					continue
				}
				// wrap the data in a message
				msg := messages.Message{
					Header:  item.value.Msg.Header,
					Message: encrypted,
				}

				r.uplink.Send(msg)
				r.updateTimer()
			}
			r.lock.Unlock()

		case <-r.ctx.Done():
			fmt.Println("Context done")
			return
		}
	}
}

// heap.Interface implementation

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	// return pq[i].value.Rto.Before(pq[j].value.Rto)
	return pq[i].value.Seq < pq[j].value.Seq
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}
