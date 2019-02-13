package actors

import (
	"fmt"
	"path"
)

type actorMessage struct {
	message interface{}
	sender  *ActorRef
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
	rootContext    actorContextImpl
}

type actorCreateRequest struct {
	name            string
	parent          *ActorRef
	factoryFunction func() Actor
	responseChannel chan<- *ActorRef
}

type actorLookupRequest struct {
	name            string
	responseChannel chan<- *ActorRef
}

func (system *ActorSystem) lookupRefBackend(name string) *ActorRef {
	impl, ok := system.registry[name]
	if ok {
		ref := new(ActorRef)
		ref.name = name
		ref.messageChannel = impl.messageChannel
		return ref
	}
	return nil
}

func (system *ActorSystem) createActor(name string, request actorCreateRequest) *ActorRef {
	var impl = actorImpl{
		path:           name,
		messageChannel: make(chan actorMessage, 10),
		actorImpl:      request.factoryFunction(),

		// Memory is owned by go thread below
		context: actorContextImpl{
			parent:               request.parent,
			children:             make(map[string]*ActorRef),
			self:                 nil,
			sender:               nil,
			systemControlChannel: system.controlChannel,
		},
	}
	system.registry[name] = impl

	var actorRef = new(ActorRef)
	actorRef.name = name
	actorRef.messageChannel = impl.messageChannel
	impl.context.self = actorRef

	// Owned by the new actor
	go (func() {
		ptrToContext := &impl.context
		defer close(impl.messageChannel)

		impl.actorImpl.OnStart(ptrToContext)
		for actorMsg := range impl.messageChannel {
			ptrToContext.sender = actorMsg.sender
			impl.actorImpl.Receive(ptrToContext, actorMsg.message)
			ptrToContext.sender = nil
		}
		impl.actorImpl.OnStop()
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

	impl := system.registry[rootRef.Name()]
	context := &impl.context

	go (func() {
		fmt.Printf("Starting actor system %s\n", system.name)

		// Create the root actor to be the parent of all actors
		for msg := range system.controlChannel {
			switch msg.(type) {
			case actorLookupRequest:
				var request = msg.(actorLookupRequest)
				var ref = system.lookupRefBackend(request.name)
				request.responseChannel <- ref
				break
			case actorCreateRequest:
				var request = msg.(actorCreateRequest)
				var name = request.name

				if request.parent == nil {
					request.parent = rootRef
				}

				name = path.Join(request.parent.Name(), name)

				_, ok := system.registry[name]
				if ok {
					// Actor already exists - send back nil
					request.responseChannel <- nil
				} else {
					ref := system.createActor(name, request)

					// If the parent is the root, we don't need to send a response,
					// we should just update the list of children here and return
					request.responseChannel <- ref
				}
				break
			default:
				fmt.Printf("Unknown control request %v", msg)
			}
		}
	})()

	return context
}

type rootActor struct{}

func (root *rootActor) OnStart(ctxt ActorContext) {
}
func (root *rootActor) OnStop() {
}
func (root *rootActor) Receive(ctt ActorContext, message interface{}) {
}

func NewSystem(name string) ActorContext {
	system := new(ActorSystem)
	system.name = name
	system.registry = make(map[string]actorImpl)
	system.controlChannel = make(chan interface{})

	// Start the system to receive control messages (necessary for actor start)
	rootContext := system.start()
	return rootContext
}
