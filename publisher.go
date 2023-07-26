// package pubsub implements the pubsub design pattern.
package pubsub

import (
	"sync"
	"time"
)

// Publisher is a basic pub/sub structure, Allows to publish and subscribe subjects.
//
// A Publisher is safe for use by multiple goroutines simultaneously.
type Publisher struct {
	mu          sync.Mutex // protects follows
	subscribers map[chan any]func(any) bool

	timeout time.Duration
	buffer  int
	closed  bool
}

// NewPublisher creates a new pub/sub publisher to deliver subjects.
// The duration is used as the send timeout as to not block the publisher deliver
// subjects to other subscribers if one subscriber not ready to receive.
// The buffer is used when creating new channel for subscribers.
func NewPublisher(timeout time.Duration, buffer int) *Publisher {
	return &Publisher{
		subscribers: make(map[chan any]func(any) bool),
		timeout:     timeout,
		buffer:      buffer,
	}
}

// Subscribe adds a new subscriber to the publisher returning the channel.
func (p *Publisher) Subscribe() (chan any, error) {
	return p.SubscribeSubjectWithBuffer(nil, p.buffer)
}

// SubscribeSubject adds a new subscriber that a subject will filter by function f.
func (p *Publisher) SubscribeSubject(f func(any) bool) (chan any, error) {
	return p.SubscribeSubjectWithBuffer(f, p.buffer)
}

// SubscribeSubjectWithBuffer adds a new subscriber that a subject will filter by function f.
// The returned channel has a buffer of the specified size.
func (p *Publisher) SubscribeSubjectWithBuffer(f func(any) bool, buffer int) (chan any, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrClosedPublisher
	}

	c := make(chan any, buffer)
	p.subscribers[c] = f
	p.mu.Unlock()
	return c, nil
}

// Unsubscribe removes the specified subscriber from the publisher.
// Unsubscribe does close the channel.
func (p *Publisher) Unsubscribe(c chan any) {
	p.mu.Lock()
	if _, ok := p.subscribers[c]; ok {
		delete(p.subscribers, c)
		close(c)
	}
	p.mu.Unlock()
}

var timerPool = sync.Pool{
	New: func() any {
		t := time.NewTimer(1<<63 - 1)
		t.Stop()
		return t
	},
}

func (p *Publisher) deliverSubject(c chan any, subject any, wg *sync.WaitGroup) {
	defer wg.Done()

	if p.timeout > 0 {
		t := timerPool.Get().(*time.Timer)
		defer timerPool.Put(t)
		t.Reset(p.timeout)
		// No defer, as we don't know which
		// case will be selected

		select {
		case <-t.C:
			// C is drained, early return
			return
		case c <- subject:
		}

		// We still need to check the return value
		// of Stop, because t could have fired
		// between the send on c and this line.
		if !t.Stop() {
			<-t.C
		}
		return
	}

	select {
	case c <- subject:
	default:
	}
}

type subscriber struct {
	c chan any
	f func(any) bool
}

var wgPool = sync.Pool{
	New: func() any {
		return new(sync.WaitGroup)
	},
}

// Publish deliver the subject to all subscribers currently registered with the publisher.
func (p *Publisher) Publish(subject any) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrClosedPublisher
	}

	if len(p.subscribers) == 0 {
		p.mu.Unlock()
		return nil
	}

	wg := wgPool.Get().(*sync.WaitGroup)
	defer wgPool.Put(wg)
	for c, f := range p.subscribers {
		if f != nil && !f(subject) {
			continue
		}

		wg.Add(1)
		go p.deliverSubject(c, subject, wg)
	}
	wg.Wait()

	p.mu.Unlock()
	return nil
}

// Len returns the number of subscribers currently registered with the publisher.
func (p *Publisher) Len() int {
	p.mu.Lock()
	n := len(p.subscribers)
	p.mu.Unlock()
	return n
}

// ErrClosedPublisher is the error used for publish or subscribe operations on a closed publisher.
var ErrClosedPublisher = closedPublisherErr{}

type closedPublisherErr struct{}

func (closedPublisherErr) Error() string { return "publisher is already closed" }

// Close closes the channels to all subscribers registered with the publisher.
func (p *Publisher) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}

	p.closed = true
	subscribers := p.subscribers
	p.subscribers = nil
	p.mu.Unlock()

	for sub := range subscribers {
		close(sub)
	}
}
