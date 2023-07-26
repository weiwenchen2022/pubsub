package pubsub

import (
	"fmt"
	"testing"
	"time"
)

func TestPublishSingle(t *testing.T) {
	t.Parallel()

	p := NewPublisher(100*time.Millisecond, 10)
	c, _ := p.Subscribe()

	go func() {
		time.Sleep(100 * time.Millisecond)
		p.Publish("hello")
	}()

	if msg := <-c; msg != "hello" {
		t.Errorf(`got %#v, want "hello"`, msg)
	}
}

func TestPublishMultiple(t *testing.T) {
	t.Parallel()

	p := NewPublisher(100*time.Millisecond, 10)

	const n = 5
	subs := make([]chan any, 0, n)
	for i := 0; i < n; i++ {
		sub, _ := p.Subscribe()
		subs = append(subs, sub)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		p.Publish("hello")
	}()

	for _, c := range subs {
		if msg := <-c; msg != "hello" {
			t.Errorf(`got %#v, want "hello"`, msg)
		}
	}
}

func TestUnsubscribe(t *testing.T) {
	t.Parallel()

	p := NewPublisher(100*time.Millisecond, 10)
	c1, _ := p.Subscribe()
	c2, _ := p.Subscribe()

	p.Unsubscribe(c1)
	p.Publish("hello")

	valid := true
	select {
	case _, valid = <-c1:
	default:
	}
	if valid {
		t.Error("expected c1 to be closed")
	}

	if msg := <-c2; msg != "hello" {
		t.Errorf(`got %#v, want "hello"`, msg)
	}
}

func TestClosePublisher(t *testing.T) {
	t.Parallel()

	p := NewPublisher(100*time.Millisecond, 10)
	const n = 5
	subs := make([]chan any, 0, 5)
	for i := 0; i < n; i++ {
		sub, _ := p.Subscribe()
		subs = append(subs, sub)
	}

	closed := make(chan struct{})
	go func() { p.Close(); close(closed) }()
	<-closed

	for _, c := range subs {
		valid := true
		select {
		case _, valid = <-c:
		default:
		}
		if valid {
			t.Error("expected all subscriber channels to be closed")
		}
	}
}

const sampleText = "test"

type testSubscriber struct {
	datac  chan any
	errorc chan error
}

func (s *testSubscriber) Err() error { return <-s.errorc }

func newTestSubscriber(p *Publisher) *testSubscriber {
	c, _ := p.Subscribe()
	ts := &testSubscriber{
		datac:  c,
		errorc: make(chan error, 1),
	}

	go func() {
		for data := range ts.datac {
			if sampleText != data {
				ts.errorc <- fmt.Errorf("Unexpected data %T (%#[1]v)", data)
				return
			}
		}
		ts.errorc <- nil
	}()

	return ts
}

// for testing with -race
func TestPubSubRace(t *testing.T) {
	t.Parallel()

	p := NewPublisher(100*time.Millisecond, 1000)

	const n = 100
	subs := make([]*testSubscriber, 0, n)
	for i := 0; i < n; i++ {
		subs = append(subs, newTestSubscriber(p))
	}

	go func() {
		for i := 0; i < 1000; i++ {
			_ = p.Publish(sampleText)
		}
	}()

	go func() {
		time.Sleep(1 * time.Second)
		p.Close()
	}()

	for _, s := range subs {
		if err := s.Err(); err != nil {
			t.Error(err)
		}
	}
}

func BenchmarkPubSub(b *testing.B) {
	const n = 50

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		p := NewPublisher(100*time.Millisecond, 1000)

		subs := make([]*testSubscriber, 0, n)
		for j := 0; j < n; j++ {
			subs = append(subs, newTestSubscriber(p))
		}

		b.StartTimer()
		for j := 0; j < 1000; j++ {
			p.Publish(sampleText)
		}

		go func() {
			time.Sleep(1 * time.Second)
			p.Close()
		}()

		for _, s := range subs {
			if err := s.Err(); err != nil {
				b.Fatal(err)
			}
		}
	}
}
