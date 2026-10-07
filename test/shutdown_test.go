// Copyright 2019 Calvin Grunewald. All rights reserved.

package test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cgrunewald/goactors"
)

// slowCountingActor sleeps on every message and counts how many it processed
type slowCountingActor struct {
	goactors.DefaultActor
	processed *int32
	stopped   *int32
}

func (a *slowCountingActor) Receive(context goactors.ActorContext, message interface{}) {
	time.Sleep(time.Millisecond)
	atomic.AddInt32(a.processed, 1)
}

func (a *slowCountingActor) OnStop() {
	atomic.AddInt32(a.stopped, 1)
}

func waitForDone(t *testing.T, system *goactors.ActorSystem) {
	t.Helper()
	select {
	case <-system.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Actor system did not shut down")
	}
}

func TestShutdownDrainsMailboxes(t *testing.T) {
	const messageCount = 50

	var processed, stopped int32
	system := goactors.NewSystem("test")
	ctxt := system.Context()

	factory := func() goactors.Actor {
		return &slowCountingActor{processed: &processed, stopped: &stopped}
	}
	first := ctxt.CreateActorFromFunc(factory, "first")
	second := ctxt.CreateActorFromFunc(factory, "second")

	// Mailboxes are bounded, so senders block while the slow actors work through
	// them. Wait until every message has been enqueued before shutting down; most
	// of them will still be unprocessed at that point
	var wg sync.WaitGroup
	for _, ref := range []goactors.ActorRef{first, second} {
		wg.Add(1)
		go func(ref goactors.ActorRef) {
			defer wg.Done()
			for i := 0; i < messageCount; i++ {
				ref.Send(nil, i)
			}
		}(ref)
	}
	wg.Wait()

	system.Shutdown()

	if got := atomic.LoadInt32(&processed); got != 2*messageCount {
		t.Errorf("Expected %d messages processed before shutdown, got %d", 2*messageCount, got)
	}
	if got := atomic.LoadInt32(&stopped); got != 2 {
		t.Errorf("Expected 2 actors stopped, got %d", got)
	}
	waitForDone(t, system)
}

func TestShutdownIsIdempotent(t *testing.T) {
	system := goactors.NewSystem("test")
	ctxt := system.Context()
	ctxt.CreateActorFromFunc(func() goactors.Actor { return &goactors.DefaultActor{} }, "a")

	system.Shutdown()
	system.Shutdown()

	if err := system.ShutdownWithContext(context.Background()); err != nil {
		t.Errorf("Unexpected error from ShutdownWithContext after Shutdown: %v", err)
	}
	waitForDone(t, system)
}

func TestShutdownAfterRootStopped(t *testing.T) {
	system := goactors.NewSystem("test")
	ctxt := system.Context()

	ctxt.Stop(ctxt.SelfRef())
	system.Wait()

	done := make(chan struct{})
	go func() {
		system.Shutdown()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown blocked after the root actor was already stopped")
	}
}

// blockingActor blocks in Receive until release is closed
type blockingActor struct {
	goactors.DefaultActor
	entered chan struct{}
	release chan struct{}
}

func (a *blockingActor) Receive(context goactors.ActorContext, message interface{}) {
	close(a.entered)
	<-a.release
}

func TestShutdownWithContextDeadline(t *testing.T) {
	system := goactors.NewSystem("test")
	ctxt := system.Context()

	entered := make(chan struct{})
	release := make(chan struct{})
	blocker := ctxt.CreateActorFromFunc(func() goactors.Actor {
		return &blockingActor{entered: entered, release: release}
	}, "blocker")

	blocker.Send(nil, "block")
	<-entered

	deadline, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if err := system.ShutdownWithContext(deadline); err != context.DeadlineExceeded {
		t.Fatalf("Expected context.DeadlineExceeded, got %v", err)
	}

	select {
	case <-system.Done():
		t.Fatal("System shut down while an actor was still processing a message")
	default:
	}

	// Shutdown continues in the background once the actor is unblocked
	close(release)
	waitForDone(t, system)
}

func TestShutdownWithContextSuccess(t *testing.T) {
	system := goactors.NewSystem("test")
	ctxt := system.Context()
	ctxt.CreateActorFromFunc(func() goactors.Actor { return &goactors.DefaultActor{} }, "a")

	timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := system.ShutdownWithContext(timeout); err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	waitForDone(t, system)
	system.Wait()
}
