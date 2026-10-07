// Copyright 2019 Calvin Grunewald. All rights reserved.

package goactors_test

import (
	"context"
	"fmt"
	"time"

	"github.com/cgrunewald/goactors"
)

// Greeter replies to every string message with a greeting
type Greeter struct {
	goactors.DefaultActor
}

func (g *Greeter) Receive(ctxt goactors.ActorContext, message interface{}) {
	if name, ok := message.(string); ok {
		ctxt.SenderRef().Send(ctxt.SelfRef(), "Hello, "+name+"!")
	}
}

// This example creates an actor system, asks an actor a question and shuts the
// system down.
func Example() {
	system := goactors.NewSystem("example")
	ctxt := system.Context()

	greeter := ctxt.CreateActorFromFunc(func() goactors.Actor {
		return &Greeter{}
	}, "greeter")

	fmt.Println(greeter.Path())
	fmt.Println(greeter.Ask("gopher").GetResult())

	// Stopping the root actor stops all of its children and the system
	ctxt.Stop(ctxt.SelfRef())
	system.Wait()

	// Output:
	// Starting actor system example
	// /example/greeter
	// Hello, gopher!
	// Shutting down actor system
}

// Counter keeps a running total and reports it when asked
type Counter struct {
	goactors.DefaultActor
	total int
}

// GetTotal asks a Counter for its current total
type GetTotal struct{}

func (c *Counter) Receive(ctxt goactors.ActorContext, message interface{}) {
	switch msg := message.(type) {
	case int:
		c.total += msg
	case GetTotal:
		ctxt.SenderRef().Send(ctxt.SelfRef(), c.total)
	}
}

// Messages sent to an actor are processed in order, one at a time, so actor
// state does not need to be locked.
func ExampleActorRef_Send() {
	system := goactors.NewSystem("counter")
	ctxt := system.Context()

	counter := ctxt.CreateActorFromFunc(func() goactors.Actor {
		return &Counter{}
	}, "counter")

	for i := 1; i <= 10; i++ {
		counter.Send(nil, i)
	}

	// The Ask is queued behind the ten Sends above
	fmt.Println("total:", counter.Ask(GetTotal{}).GetResult())

	ctxt.Stop(ctxt.SelfRef())
	system.Wait()

	// Output:
	// Starting actor system counter
	// total: 55
	// Shutting down actor system
}

// Silent never replies
type Silent struct {
	goactors.DefaultActor
}

// GetResultWithTimeout bounds how long the caller waits for a reply.
func ExampleFuture_GetResultWithTimeout() {
	system := goactors.NewSystem("timeout")
	ctxt := system.Context()

	silent := ctxt.CreateActorFromFunc(func() goactors.Actor {
		return &Silent{}
	}, "silent")

	_, err := silent.Ask("anyone there?").GetResultWithTimeout(10 * time.Millisecond)
	fmt.Println(err == goactors.ErrFutureTimeout)

	ctxt.Stop(ctxt.SelfRef())
	system.Wait()

	// Output:
	// Starting actor system timeout
	// true
	// Shutting down actor system
}

// GetResultWithContext lets a caller abandon a wait when its context is done.
func ExampleFuture_GetResultWithContext() {
	system := goactors.NewSystem("cancel")
	ctxt := system.Context()

	silent := ctxt.CreateActorFromFunc(func() goactors.Actor {
		return &Silent{}
	}, "silent")

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := silent.Ask("anyone there?").GetResultWithContext(cancelCtx)
	fmt.Println(err)

	ctxt.Stop(ctxt.SelfRef())
	system.Wait()

	// Output:
	// Starting actor system cancel
	// context canceled
	// Shutting down actor system
}

// Parent creates a child actor when it starts
type Parent struct {
	goactors.DefaultActor
}

func (p *Parent) OnStart(ctxt goactors.ActorContext) {
	ctxt.CreateActorFromFunc(func() goactors.Actor { return &Greeter{} }, "child")
}

// Actors form a hierarchy; children are addressed by path and can be found from
// any context.
func ExampleActorContext_FindActor() {
	system := goactors.NewSystem("tree")
	ctxt := system.Context()

	ctxt.CreateActorFromFunc(func() goactors.Actor { return &Parent{} }, "parent")

	child := ctxt.FindActor("/tree/parent/child")
	fmt.Println(child.Path())
	fmt.Println(child.Ask("child").GetResult())

	ctxt.Stop(ctxt.SelfRef())
	system.Wait()

	// Output:
	// Starting actor system tree
	// /tree/parent/child
	// Hello, child!
	// Shutting down actor system
}
