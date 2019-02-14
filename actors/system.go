package actors

import (
	"fmt"
	"path"
	"sort"
	"sync"
)

type actorMessage struct {
	message interface{}
	sender  ActorRef
}

type actorImpl struct {
	messageChannel chan actorMessage
	path           string
	messageBuffer  []interface{}
	actorImpl      Actor
	context        actorContextImpl
}

type ActorSystem struct {
	registry       map[string]actorImpl
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

func (system *ActorSystem) createActor(name string, request actorCreateRequest) ActorRef {
	// Running in the context of the main system goroutine

	var impl = actorImpl{
		path:           name,
		messageChannel: make(chan actorMessage, 10),
		actorImpl:      request.factoryFunction(),

		// Memory is owned by go thread below
		context: actorContextImpl{
			parent:               request.parent,
			path:                 name,
			children:             make(map[string]ActorRef),
			self:                 nil,
			sender:               nil,
			systemControlChannel: system.controlChannel,
		},
	}

	var actorRef = new(actorRef)
	actorRef.name = name
	actorRef.messageChannel = impl.messageChannel
	impl.context.self = actorRef

	system.registry[name] = impl

	// Owned by the new actor
	go (func() {
		ptrToContext := &impl.context
		var stopChannel chan<- bool

		impl.actorImpl.OnStart(ptrToContext)

		// Once the actor is started, notify the creator
		if request.responseChannel != nil {
			request.responseChannel <- actorRef
		}

		fmt.Printf("Actor %s is now receiving messages\n", ptrToContext.path)
	loop:
		for actorMsg := range impl.messageChannel {
			ptrToContext.sender = actorMsg.sender

			switch actorMsg.message.(type) {
			case poisonPillMessage:
				fmt.Printf("Received poison pill %v\n", ptrToContext.path)
				pill := actorMsg.message.(poisonPillMessage)
				childrenResultChannel := make(chan bool)
				defer close(childrenResultChannel)

				// Sort the children so we have consistent stopping
				sortedChildren := make([]string, 0, len(ptrToContext.children))
				for k := range ptrToContext.children {
					sortedChildren = append(sortedChildren, k)
				}

				sort.Strings(sortedChildren)
				for _, val := range sortedChildren {
					ptrToContext.children[val].Send(
						ptrToContext.SelfRef(),
						poisonPillMessage{resultChannel: childrenResultChannel})
					<-childrenResultChannel
				}

				stopChannel = pill.resultChannel

				// close(impl.messageChannel) - can't close the channel since we don't know who our writers are
				// should be garbage collected at some point
				break loop
			default:
				impl.actorImpl.Receive(ptrToContext, actorMsg.message)
				ptrToContext.sender = nil
			}
		}
		ptrToContext.self = nil // self is destructed at this point
		impl.actorImpl.OnStop()

		// Have the control thread unregister the actor
		responseChannel := make(chan interface{})
		defer close(responseChannel)
		ptrToContext.systemControlChannel <- actorStopRequest{
			path:            name,
			responseChannel: responseChannel,
		}
		<-responseChannel

		// Notify the parent the child is stopped
		if stopChannel != nil {
			stopChannel <- true
		}

	})()
	return actorRef
}

func (system *ActorSystem) start() ActorContext {
	rootRef := system.createActor(
		path.Join("/", system.name),
		actorCreateRequest{
			parent: nil,
			factoryFunction: func() Actor {
				return new(rootActor)
			},
		})

	impl := system.registry[rootRef.Path()]
	context := &impl.context
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
					system.createActor(name, request)
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
	system.registry = make(map[string]actorImpl)
	system.controlChannel = make(chan interface{})
	system.waitGroup = sync.WaitGroup{}

	// Start the system to receive control messages (necessary for actor start)
	system.rootContext = system.start()
	return system
}
