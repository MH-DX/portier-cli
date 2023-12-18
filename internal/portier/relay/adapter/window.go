package adapter

import (
	"errors"
	"sync"
	"time"

	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"gopkg.in/eapache/queue.v1"
)

type WindowOptions struct {
	// initial size of the window in bytes
	InitialCap int
}

// A windowItem is an item in the window
type windowItem struct {
	msg           messages.DataMessage
	time          time.Time
	acked         bool
	retransmitted bool
}

type Window interface {
	// add is called when a message is added to the window
	// seq is the sequence number of the message
	// this function will block until there is enough space in the window
	add(msg messages.DataMessage) error

	// ack is called when a message has been ack'ed by peer
	// seq is the sequence number of the ack'ed message
	// retransmitted indicates if the message rtt was a retransmission
	// returns the rtt of the message, and a flag indicating if the message was a retransmission or influrnced by a retransmission (i.e. rtt is not accurate)
	ack(seq uint64, retransmitted bool) (rtt time.Duration, retrans_flag bool, err error)

	// setMaxSize sets the size of the window in bytes
	setCap(size int)

	// getCurSize returns the current size of the window in bytes
	getCurCap() int
}

type window struct {
	options     WindowOptions
	currentSize int
	currentCap  int
	queue       *queue.Queue
	mutex       *sync.Mutex
	cond        *sync.Cond
}

func NewWindow(options WindowOptions) Window {
	mutex := sync.Mutex{}
	return &window{
		options:     options,
		currentSize: 0,
		currentCap:  options.InitialCap,
		queue:       queue.New(),
		mutex:       &mutex,
		cond:        sync.NewCond(&mutex),
	}
}

func (w *window) add(msg messages.DataMessage) error {
	w.mutex.Lock()
	defer func() { w.mutex.Unlock() }()

	for w.currentSize+len(msg.Data) > w.currentCap {
		// wait until there is enough space in the window
		w.cond.Wait()
	}
	w.currentSize += len(msg.Data)
	w.queue.Add(&windowItem{
		msg:           msg,
		time:          time.Now(),
		acked:         false,
		retransmitted: false,
	})
	return nil
}

func (w *window) ack(seq uint64, retransmitted bool) (rtt time.Duration, retrans_flag bool, err error) {
	w.mutex.Lock()
	defer func() { w.mutex.Unlock() }()

	// determine the index of the message in the queue using the sequence number
	first := w.queue.Peek().(*windowItem).msg.Seq
	index := int(seq - first)
	if index < 0 || index >= w.queue.Length() {
		// the message is not in the window anymore - it has already been ack'ed
		return 0, false, errors.New("message_not_in_window")
	}
	// get the message from the queue
	item := w.queue.Get(index).(*windowItem)
	if item.acked {
		// the message has already been ack'ed
		return 0, false, errors.New("message_already_acked")
	}
	// mark the message as ack'ed
	item.acked = true
	rtt = time.Since(item.time)
	// if the message was a retransmission, mark it as such and all messages after it as well
	if retransmitted {
		item.retransmitted = true
		for i := index + 1; i < w.queue.Length(); i++ {
			// retrieve pointer to message
			item := w.queue.Get(i).(*windowItem)
			// mark message as retransmitted
			item.retransmitted = true
		}
	}
	// remove all messages from the queue that have been ack'ed
	for w.queue.Length() > 0 {
		item := w.queue.Peek().(*windowItem)
		if item.acked {
			w.queue.Remove()
			w.currentSize -= len(item.msg.Data)
		} else {
			break
		}
	}
	w.cond.Signal()
	return rtt, retransmitted || item.retransmitted, nil
}

func (w *window) setCap(size int) {
	w.mutex.Lock()
	defer func() { w.mutex.Unlock() }()

	w.currentCap = size
}

func (w *window) getCurCap() int {
	return w.currentSize
}
