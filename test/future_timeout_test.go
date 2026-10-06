// Copyright 2019 Calvin Grunewald. All rights reserved.

package test

import (
	"context"
	"testing"
	"time"

	"github.com/cgrunewald/goactors"
)

// silentActor never replies, so any Ask against it never completes
type silentActor struct {
	goactors.DefaultActor
}

// delayedEchoActor replies to a time.Duration message after sleeping for that long
type delayedEchoActor struct {
	goactors.DefaultActor
}

func (a *delayedEchoActor) Receive(context goactors.ActorContext, message interface{}) {
	if delay, ok := message.(time.Duration); ok {
		time.Sleep(delay)
	}
	context.SenderRef().Send(context.SelfRef(), message)
}

func TestFutureGetResultWithTimeoutSuccess(t *testing.T) {
	system := goactors.NewSystem("test")
	ctxt := system.Context()

	echo := ctxt.CreateActorFromFunc(func() goactors.Actor { return &echoActor{t: t} }, "echo")
	result, err := echo.Ask("ping").GetResultWithTimeout(time.Second)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if result.(string) != "ping" {
		t.Errorf("Expected %s, received %v", "ping", result)
	}

	ctxt.Stop(ctxt.SelfRef())
	system.Wait()
}

func TestFutureGetResultWithTimeoutExpires(t *testing.T) {
	system := goactors.NewSystem("test")
	ctxt := system.Context()

	silent := ctxt.CreateActorFromFunc(func() goactors.Actor { return &silentActor{} }, "silent")

	start := time.Now()
	result, err := silent.Ask("ping").GetResultWithTimeout(50 * time.Millisecond)
	elapsed := time.Since(start)

	if err != goactors.ErrFutureTimeout {
		t.Fatalf("Expected ErrFutureTimeout, received %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result on timeout, received %v", result)
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("Returned before timeout elapsed (%v)", elapsed)
	}

	ctxt.Stop(ctxt.SelfRef())
	system.Wait()
}

func TestFutureLateResultAfterTimeout(t *testing.T) {
	system := goactors.NewSystem("test")
	ctxt := system.Context()

	delayed := ctxt.CreateActorFromFunc(func() goactors.Actor { return &delayedEchoActor{} }, "delayed")
	future := delayed.Ask(100 * time.Millisecond)

	if _, err := future.GetResultWithTimeout(10 * time.Millisecond); err != goactors.ErrFutureTimeout {
		t.Fatalf("Expected ErrFutureTimeout, received %v", err)
	}

	// The actor must not block when replying to a timed out future, and the
	// result can still be retrieved by waiting again
	result, err := future.GetResultWithTimeout(time.Second)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if result.(time.Duration) != 100*time.Millisecond {
		t.Errorf("Unexpected result %v", result)
	}

	// Once consumed, the future behaves like GetResult and yields nil
	result, err = future.GetResultWithTimeout(10 * time.Millisecond)
	if result != nil || err != nil {
		t.Errorf("Expected (nil, nil) from consumed future, received (%v, %v)", result, err)
	}

	ctxt.Stop(ctxt.SelfRef())
	system.Wait()
}

func TestFutureGetResultWithContextSuccess(t *testing.T) {
	system := goactors.NewSystem("test")
	ctxt := system.Context()

	echo := ctxt.CreateActorFromFunc(func() goactors.Actor { return &echoActor{t: t} }, "echo")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := echo.Ask("ping").GetResultWithContext(ctx)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if result.(string) != "ping" {
		t.Errorf("Expected %s, received %v", "ping", result)
	}

	ctxt.Stop(ctxt.SelfRef())
	system.Wait()
}

func TestFutureGetResultWithContextCancelled(t *testing.T) {
	system := goactors.NewSystem("test")
	ctxt := system.Context()

	silent := ctxt.CreateActorFromFunc(func() goactors.Actor { return &silentActor{} }, "silent")
	future := silent.Ask("ping")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	result, err := future.GetResultWithContext(ctx)
	if err != context.Canceled {
		t.Fatalf("Expected context.Canceled, received %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result on cancel, received %v", result)
	}

	ctxt.Stop(ctxt.SelfRef())
	system.Wait()
}

func TestFutureGetResultWithContextDeadline(t *testing.T) {
	system := goactors.NewSystem("test")
	ctxt := system.Context()

	silent := ctxt.CreateActorFromFunc(func() goactors.Actor { return &silentActor{} }, "silent")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if _, err := silent.Ask("ping").GetResultWithContext(ctx); err != context.DeadlineExceeded {
		t.Fatalf("Expected context.DeadlineExceeded, received %v", err)
	}

	ctxt.Stop(ctxt.SelfRef())
	system.Wait()
}
