// Copyright 2019 Calvin Grunewald. All rights reserved.

/*
Package goactors is a small actor-model library for Go.

An actor is a value implementing the Actor interface. Each actor runs on its own
goroutine and processes the messages in its mailbox one at a time, so an actor's
state never needs to be locked as long as it is only touched from OnStart,
Receive and OnStop.

# Actor systems

Actors live in an ActorSystem created with NewSystem. Every system has a root
actor whose context is returned by ActorSystem.Context; actors created from the
root context become children of the root actor:

	system := goactors.NewSystem("app")
	ctxt := system.Context()
	worker := ctxt.CreateActorFromFunc(func() goactors.Actor { return &Worker{} }, "worker")

Actors are addressed by hierarchical paths such as "/app/worker" and can be
looked up with ActorContext.FindActor. Child actors are created from within an
actor using the ActorContext passed to OnStart and Receive.

# Messaging

Messages are arbitrary values. ActorRef.Send delivers a message asynchronously,
optionally recording the sender so that the receiver can reply via
ActorContext.SenderRef. ActorRef.Ask sends a message and returns a Future that is
completed by the receiver's reply; the result can be awaited with GetResult,
GetResultWithTimeout or GetResultWithContext, or forwarded to another actor with
ForwardResult.

Actors created with CreateProxyActorFromFunc are fronted by an unbounded buffer,
so senders never block on a full mailbox.

# Lifecycle

ActorContext.Stop sends a poison pill to an actor. Messages already in the
actor's mailbox are processed first; the actor then stops its children (in
name order), runs OnStop and is unregistered. Stopping the root actor
(ctxt.Stop(ctxt.SelfRef())) shuts down the whole system, and ActorSystem.Wait
blocks until that has completed.

Embed DefaultActor to get no-op implementations of OnStart, OnStop and Receive.
*/
package goactors
