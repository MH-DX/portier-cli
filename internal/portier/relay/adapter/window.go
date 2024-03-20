package adapter

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/marinator86/portier-cli/internal/portier/relay/adapter/rto_heap"
	"github.com/marinator86/portier-cli/internal/portier/relay/adapter/rtt"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/encryption"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
	windowitem "github.com/marinator86/portier-cli/internal/portier/relay/window_item"
	"gopkg.in/eapache/queue.v1"
)

type WindowOptions struct {
	// initial size of the window in bytes
	InitialCap float64

	// minimum rto of the window in microseconds
	MinRTO float64

	// initial rto of the window in microseconds
	InitialRTO float64

	// rtt factor for calculating the rto
	RTTFactor float64 // 4.0

	// ewma alpha
	EWMAAlpha float64 // 0.125

	// ewma beta
	EWMABeta float64 // 0.25

	// MaxCap is the maximum size of the window in bytes
	MaxCap float64

	// WindowDownscaleFactor is the factor by which the window is downscaled when a retransmission is detected
	WindowDownscaleFactor float64

	// WindowUpscaleFactor is the factor by which the window is upscaled when a retransmission is not detected
	WindowUpscaleFactor float64

	// HistSize is the size of the sliding window histogram
	RTTHistSize int

	// BaseRTT calculation interval in number of messages
	BaseRTTInterval uint64

	// BaseRTT initial window size in number of messages
	BaseRTTInitPhase uint64
}

type Window interface {
	// add is called when a message is added to the window
	// seq is the sequence number of the message
	// this function will block until there is enough space in the window
	add(msg messages.Message, seq uint64) error

	// ack is called when a message has been ack'ed by peer
	// seq is the sequence number of the ack'ed message
	// retransmitted indicates if the message rtt was a retransmission
	// returns the rtt of the message, and a flag indicating if the message was a retransmission or influrnced by a retransmission (i.e. rtt is not accurate)
	ack(seq uint64, retransmitted bool) error
}

type window struct {
	options        WindowOptions
	currentSize    int
	currentCap     float64
	currentBaseRTT float64
	queue          *queue.Queue
	mutex          *sync.Mutex
	cond           *sync.Cond
	uplink         uplink.Uplink
	stats          rtt.TCPStats
	rtoHeap        rto_heap.RtoHeap
}

func NewDefaultWindowOptions() WindowOptions {
	return WindowOptions{
		InitialCap:            32768 * 4,
		MinRTO:                5000000.0,
		InitialRTO:            300000000.0,
		RTTFactor:             4.0,
		EWMAAlpha:             0.125,
		EWMABeta:              0.25,
		MaxCap:                32768 * 32,
		WindowDownscaleFactor: 0.5,
		WindowUpscaleFactor:   1.5,
		RTTHistSize:           200,
		BaseRTTInterval:       100,
		BaseRTTInitPhase:      25,
	}
}

func NewWindow(ctx context.Context, options WindowOptions, uplink uplink.Uplink, encoderDecoder encoder.EncoderDecoder, encryption encryption.Encryption) Window {
	mutex := sync.Mutex{}
	return &window{
		options:        options,
		currentSize:    0,
		currentCap:     options.InitialCap,
		currentBaseRTT: 0,
		queue:          queue.New(),
		mutex:          &mutex,
		cond:           sync.NewCond(&mutex),
		uplink:         uplink,
		stats:          rtt.NewTCPStats(options.InitialRTO, options.EWMAAlpha, options.EWMABeta, options.MinRTO, options.RTTFactor, 2*options.RTTHistSize),
		rtoHeap:        rto_heap.NewRtoHeap(ctx, rto_heap.NewDefaultRtoHeapOptions(), uplink, encoderDecoder, encryption),
	}
}

func newWindow(ctx context.Context, options WindowOptions, uplink uplink.Uplink, rtoHeap rto_heap.RtoHeap) Window {
	mutex := sync.Mutex{}
	return &window{
		options:        options,
		currentSize:    0,
		currentCap:     options.InitialCap,
		currentBaseRTT: 0,
		queue:          queue.New(),
		mutex:          &mutex,
		cond:           sync.NewCond(&mutex),
		uplink:         uplink,
		stats:          rtt.NewTCPStats(options.InitialRTO, options.EWMAAlpha, options.EWMABeta, options.MinRTO, options.RTTFactor, 2*options.RTTHistSize),
		rtoHeap:        rtoHeap,
	}
}

func (w *window) add(msg messages.Message, seq uint64) error {
	w.mutex.Lock()
	defer func() { w.mutex.Unlock() }()

	for w.currentSize+len(msg.Message) > int(w.currentCap) {
		// wait until there is enough space in the window
		w.cond.Wait()
	}
	w.currentSize += len(msg.Message)
	now := time.Now()
	rtoDuration := time.Duration(w.stats.RTO) * time.Nanosecond
	item := &windowitem.WindowItem{
		Msg:           msg,
		Seq:           seq,
		Time:          now,
		Rto:           now.Add(rtoDuration),
		Acked:         false,
		Retransmitted: false,
		RtoDuration:   rtoDuration,
	}
	w.queue.Add(item)
	_ = w.rtoHeap.Add(item)
	_ = w.uplink.Send(msg)
	return nil
}

func (w *window) ack(seq uint64, retransmitted bool) error {
	w.mutex.Lock()
	defer func() { w.mutex.Unlock() }()

	if w.queue.Length() == 0 {
		// the window is empty
		return errors.New("window_empty")
	}

	// determine the index of the message in the queue using the sequence number
	first := w.queue.Peek().(*windowitem.WindowItem).Seq
	index := int(seq - first)
	if index < 0 || index >= w.queue.Length() {
		// the message is not in the window anymore - it has already been ack'ed
		return errors.New("message_not_in_window")
	}
	// get the message from the queue
	item := w.queue.Get(index).(*windowitem.WindowItem)
	if item.Acked {
		// the message has already been ack'ed
		return errors.New("message_already_acked")
	}

	// mark the message as ack'ed
	item.Acked = true
	defer func() { w.cond.Signal() }()
	if retransmitted {
		// if the message was a retransmission, mark it as such and all messages after it as well
		item.Retransmitted = true
		for i := index + 1; i < w.queue.Length(); i++ {
			// retrieve pointer to message
			item := w.queue.Get(i).(*windowitem.WindowItem)
			// mark message as retransmitted
			item.Retransmitted = true
		}
	} else if !item.Retransmitted {
		rtt := float64(time.Since(item.Time))
		if seq < w.options.BaseRTTInitPhase {
			if !w.stats.IsInitialized() {
				w.stats.Init(rtt)
			}
			w.currentBaseRTT = w.stats.GetBaseRTT()
		} else if seq%w.options.BaseRTTInterval == 0 {
			w.currentBaseRTT = w.stats.GetBaseRTT()
		}

		w.stats.UpdateRTT(rtt)
		if w.currentBaseRTT < w.stats.SRTT-w.stats.RTTVAR {
			w.currentCap = math.Max(w.currentCap*w.options.WindowDownscaleFactor, w.options.InitialCap)
		} else {
			w.currentCap = math.Min(w.currentCap*w.options.WindowUpscaleFactor, w.options.MaxCap)
		}
	}
	// remove all messages from the queue that have been ack'ed
	for w.queue.Length() > 0 {
		item := w.queue.Peek().(*windowitem.WindowItem)
		if item.Acked {
			w.queue.Remove()
			w.currentSize -= len(item.Msg.Message)
		} else {
			break
		}
	}
	return nil
}
