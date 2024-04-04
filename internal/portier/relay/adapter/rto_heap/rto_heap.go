package rto_heap

import (
	"container/heap"
	"context"
	"errors"
	"log"
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
	ticker        *time.Ticker
	updateChannel chan bool
	ctx           context.Context
	lock          sync.Mutex
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
		ticker:        time.NewTicker(time.Millisecond * 20),
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
	r.lock.Unlock()
	return nil
}

func (r *rtoHeap) process() {
	for {

		select {
		case <-r.ticker.C:
			// Ticker expired, check the items and resend if necessary

			// iterate over every item in the queue and remove it when it is acked
			// or resend it when it is not acked
			if len(r.queue) == 0 {
				continue
			}

			r.lock.Lock()
			for i := 0; i < len(r.queue); i++ {
				item := r.queue[i].value
				if item.Acked {
					heap.Remove(&r.queue, i)
					i--
					continue
				}
				if item.Rto.Before(time.Now()) {
					// resend the message
					log.Printf("Resending message: %d, timeout: %d ms\n", item.Seq, item.RtoDuration.Abs().Milliseconds())
					// decode the datamessage and update the retransmitted flag
					dataMsg, err := r.encoder.DecodeDataMessage(item.Msg.Message)
					if err != nil {
						log.Println("Error decoding data message")
						continue
					}
					dataMsg.Re = true
					// encode the datamessage
					dmBytes, err := r.encoder.EncodeDataMessage(dataMsg)
					if err != nil {
						log.Println("Error encoding data message")
						continue
					}
					// encrypt the data
					encrypted, err := r.encryption.Encrypt(item.Msg.Header, dmBytes)
					if err != nil {
						log.Printf("error encrypting data: %s\n", err)
						continue
					}
					// wrap the data in a message
					msg := messages.Message{
						Header:  item.Msg.Header,
						Message: encrypted,
					}

					err = r.uplink.Send(msg)
					if err != nil {
						log.Printf("Error sending message: %s\n", err)
					}

					item.Rto = time.Now().Add(item.RtoDuration)
				}
			}
			r.lock.Unlock()

		case <-r.ctx.Done():
			log.Printf("RTO heap shutting down")
			return
		}
	}
}

// heap.Interface implementation

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return pq[i].value.Rto.Before(pq[j].value.Rto)
	// return pq[i].value.Seq < pq[j].value.Seq
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
