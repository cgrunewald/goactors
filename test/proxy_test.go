// Copyright 2019 Calvin Grunewald. All rights reserved.

package test

import (
	"testing"
	"time"

	"github.com/cgrunewald/goactors"
)

// selfReportingActor captures the self ref it observes in OnStart and replies to any
// message with the self ref it observes in Receive
type selfReportingActor struct {
	goactors.DefaultActor
	startSelf chan goactors.ActorRef
}

func (a *selfReportingActor) OnStart(context goactors.ActorContext) {
	a.startSelf <- context.SelfRef()
}

func (a *selfReportingActor) Receive(context goactors.ActorContext, message interface{}) {
	context.SenderRef().Send(context.SelfRef(), context.SelfRef())
}

func TestProxyActorSelfRefIsProxy(t *testing.T) {
	system := goactors.NewSystem("test")
	ctxt := system.Context()

	startSelf := make(chan goactors.ActorRef, 1)
	ref := ctxt.CreateProxyActorFromFunc(func() goactors.Actor {
		return &selfReportingActor{startSelf: startSelf}
	}, "proxy")

	if ref == nil {
		t.Fatal("Expected proxy actor to be created")
	}

	if observed := <-startSelf; observed != ref {
		t.Errorf("SelfRef in OnStart (%p) does not match ref returned to creator (%p)", observed, ref)
	}

	result, err := ref.Ask("who").GetResultWithTimeout(time.Second)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if result.(goactors.ActorRef) != ref {
		t.Errorf("SelfRef in Receive (%p) does not match ref returned to creator (%p)", result, ref)
	}

	if found := ctxt.FindActor("/test/proxy"); found != ref {
		t.Errorf("FindActor returned %p, expected proxy ref %p", found, ref)
	}

	ctxt.Stop(ctxt.SelfRef())
	system.Wait()
}

// countingActor counts messages and reports the count on request
type countingActor struct {
	goactors.DefaultActor
	count int
}

type countRequest struct{}

func (a *countingActor) Receive(context goactors.ActorContext, message interface{}) {
	switch message.(type) {
	case countRequest:
		context.SenderRef().Send(context.SelfRef(), a.count)
	default:
		a.count++
	}
}

func TestProxyActorBuffersBurstAndStops(t *testing.T) {
	const messageCount = 1000

	system := goactors.NewSystem("test")
	ctxt := system.Context()

	ref := ctxt.CreateProxyActorFromFunc(func() goactors.Actor {
		return &countingActor{}
	}, "counter")

	// Far more messages than the actor's mailbox can hold; the proxy must buffer them
	// without blocking the sender
	done := make(chan struct{})
	go func() {
		for i := 0; i < messageCount; i++ {
			ref.Send(nil, i)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Sending to a proxy actor blocked")
	}

	result, err := ref.Ask(countRequest{}).GetResultWithTimeout(5 * time.Second)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if result.(int) != messageCount {
		t.Errorf("Expected %d messages, received %v", messageCount, result)
	}

	ctxt.Stop(ctxt.SelfRef())
	system.Wait()
}
