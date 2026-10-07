// Copyright 2019 Calvin Grunewald. All rights reserved.

package goactors

import (
	"context"
	"fmt"
	"path"
	"sync"
)

type actorMessage struct {
	message interface{}
	sender  ActorRef
}

type ActorSystem struct {
	// Used by Shutdown/Done; set up once in NewSystem
	rootRef      ActorRef
	shutdownOnce sync.Once
	done         chan struct{}

	registry       map[string]*actorImpl
	name           string
	controlChannel chan interface{}
	rootContext    ActorContext
	waitGroup      sync.WaitGroup
}

type actorStopRequest struct {
	responseChannel chan<- interface{}
	path            string
}

type actorCreateRequest struct {
	name            string
	proxy           bool
	parent          ActorRef
	factoryFunction func() Actor
	responseChannel chan<- ActorRef
}

type actorLookupRequest struct {
	name            string
	responseChannel chan<- ActorRef
}

type poisonPillMessage struct {
	resultChannel chan<- bool
}

func (system *ActorSystem) lookupRefBackend(name string) ActorRef {
	impl, ok := system.registry[name]
	if ok {
		return impl.context.self
	}
	return nil
}

func (system *ActorSystem) start() ActorContext {
	rootImpl := newActor(
		path.Join("/", system.name),
		system.controlChannel,
		actorCreateRequest{
			parent: nil,
			factoryFunction: func() Actor {
				return new(rootActor)
			},
		})

	system.registry[rootImpl.path] = rootImpl
	context := &rootImpl.context
	rootRef := context.self
	system.waitGroup.Add(1)

	go (func() {
		fmt.Printf("Starting actor system %s\n", system.name)

		// Create the root actor to be the parent of all actors
	loop:
		for msg := range system.controlChannel {
			switch msg.(type) {
			case actorLookupRequest:
				var request = msg.(actorLookupRequest)
				var ref = system.lookupRefBackend(request.name)
				request.responseChannel <- ref
				break
			case actorStopRequest:
				var request = msg.(actorStopRequest)
				delete(system.registry, request.path)
				request.responseChannel <- true

				if request.path == rootRef.Path() {
					break loop
				}
				break
			case actorCreateRequest:
				var request = msg.(actorCreateRequest)
				var name = request.name

				if request.parent == nil {
					request.parent = rootRef
				}

				name = path.Join(request.parent.Path(), name)

				_, ok := system.registry[name]
				if ok {
					// Actor already exists - send back nil
					request.responseChannel <- nil
				} else {
					actorImpl := newActor(name, system.controlChannel, request)
					system.registry[name] = actorImpl
				}
				break
			default:
				fmt.Printf("Unknown control request %v", msg)
			}
		}

		system.waitGroup.Done()
		fmt.Println("Shutting down actor system")
	})()

	return context
}

type rootActor struct {
	DefaultActor
}

func (system *ActorSystem) IsRunning() bool {
	return len(system.registry) > 0
}

func (system *ActorSystem) Context() ActorContext {
	return system.rootContext
}

func (system *ActorSystem) Wait() {
	system.waitGroup.Wait()
}

func NewSystem(name string) *ActorSystem {
	system := new(ActorSystem)
	system.name = name
	system.registry = make(map[string]*actorImpl)
	system.controlChannel = make(chan interface{})
	system.waitGroup = sync.WaitGroup{}

	// Start the system to receive control messages (necessary for actor start)
	system.rootContext = system.start()
	system.rootRef = system.rootContext.SelfRef()

	system.done = make(chan struct{})
	go func() {
		system.waitGroup.Wait()
		close(system.done)
	}()

	return system
}

// Done returns a channel that is closed once the actor system has fully shut down,
// i.e. after the root actor and all of its descendants have stopped.
func (system *ActorSystem) Done() <-chan struct{} {
	return system.done
}

// initiateShutdown sends a poison pill to the root actor exactly once. Because the
// pill is queued behind any messages already sent, every actor finishes processing
// its mailbox before it stops; children are stopped before their parents.
func (system *ActorSystem) initiateShutdown() {
	system.shutdownOnce.Do(func() {
		select {
		case <-system.done:
			// Already shut down (e.g. the root actor was stopped directly)
			return
		default:
		}

		go system.rootRef.Send(nil, poisonPillMessage{resultChannel: nil})
	})
}

// Shutdown gracefully stops the actor system and blocks until it has shut down.
// Messages sent before Shutdown is called are processed before each actor stops.
// It is safe to call Shutdown more than once, and after the root actor has been
// stopped by other means.
func (system *ActorSystem) Shutdown() {
	system.initiateShutdown()
	<-system.done
}

// ShutdownWithContext is like Shutdown but stops waiting when ctx is done, returning
// ctx.Err(). The shutdown itself is not cancelled and continues in the background;
// use Done or Wait to observe its completion.
func (system *ActorSystem) ShutdownWithContext(ctx context.Context) error {
	system.initiateShutdown()

	select {
	case <-system.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
