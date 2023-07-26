# pubsub

package pubsub implements the pubsub design pattern.

## Install

```sh
go get github.com/weiwenchen2022/pubsub
```

## Example

```go
package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/weiwenchen2022/pubsub"
)

func main() {
	p := pubsub.NewPublisher(100*time.Millisecond, 10)
	defer p.Close()

	protobuf, _ := p.SubscribeSubject(func(subject any) bool {
		s, ok := subject.(string)
		if !ok {
			return false
		}
		return strings.Contains(s, "protobuf")
	})
	grpc, _ := p.SubscribeSubject(func(subject any) bool {
		s, ok := subject.(string)
		if !ok {
			return false
		}
		return strings.Contains(s, "grpc")
	})

	go func() {
		p.Publish("world")
		time.Sleep(100 * time.Millisecond)
		p.Publish("https://grpc.io")
	}()

	go func() {
		p.Publish("hello")
		time.Sleep(100 * time.Millisecond)
		p.Publish("https://protobuf.dev")
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		fmt.Println("protobuf subject:", <-protobuf)
		wg.Done()
	}()

	go func() {
		fmt.Println("grpc subject:", <-grpc)
		wg.Done()
	}()

	wg.Wait()

	// Unordered output:
	// protobuf subject: https://protobuf.dev
	// grpc subject: https://grpc.io
}
```

## Reference

GoDoc: [https://godoc.org/github.com/weiwenchen2022/pubsub](https://godoc.org/github.com/weiwenchen2022/pubsub)
