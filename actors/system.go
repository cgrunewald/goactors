package actors

import (
	"fmt"
	"path"
	"sync"
)

type actorMessage struct {
	message interface{}
	sender  ActorRef
}

type ActorSystem struct {
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
	return system
}
