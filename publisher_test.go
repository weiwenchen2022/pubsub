package pubsub

import (
	"fmt"
	"testing"
	"time"
)

func TestSendToOneSub(t *testing.T) {
	t.Parallel()

	p := NewPublisher(100*time.Millisecond, 10)
	c := p.Subscribe()

	p.Publish("hello")
	if got := <-c; got != "hello" {
		t.Errorf(`got %#v, want "hello"`, got)
	}
}

func TestSendToMultipleSubs(t *testing.T) {
	t.Parallel()

	p := NewPublisher(100*time.Millisecond, 10)

	const n = 5
	subs := make([]chan any, 0, n)
	for i := 0; i < n; i++ {
		subs = append(subs, p.Subscribe())
	}

	p.Publish("hello")

	for _, c := range subs {
		if got := <-c; got != "hello" {
			t.Errorf(`got %#v, want "hello"`, got)
		}
	}
}

func TestEvictOneSub(t *testing.T) {
	t.Parallel()

	p := NewPublisher(100*time.Millisecond, 10)
	c1, c2 := p.Subscribe(), p.Subscribe()

	p.Unsubscribe(c1)
	p.Publish("hello")

	select {
	case <-c1:
		t.Error("expected c1 to not receive the published message")
	default:
	}

	if got := <-c2; got != "hello" {
		t.Errorf(`got %#v, want "hello"`, got)
	}
}

func TestClosePublisher(t *testing.T) {
	t.Parallel()

	p := NewPublisher(100*time.Millisecond, 10)
	const n = 5
	subs := make([]chan any, 0, 5)
	for i := 0; i < n; i++ {
		subs = append(subs, p.Subscribe())
	}

	p.Close()

	for _, c := range subs {
		opened := true
		select {
		case _, opened = <-c:
		default:
		}
		if opened {
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
	ts := &testSubscriber{
		datac:  p.Subscribe(),
		errorc: make(chan error, 1),
	}

	go func() {
		defer close(ts.errorc)
		for {
			select {
			case data, ok := <-ts.datac:
				if !ok {
					return
				}

				if sampleText != data {
					ts.errorc <- fmt.Errorf("Unexpected data %T (%#[1]v)", data)
					return
				}
			case <-time.After(100 * time.Millisecond):
				return
			}
		}
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
	for i := 0; i < 1000; i++ {
		p.Publish(sampleText)
	}

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
