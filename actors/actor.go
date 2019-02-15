package actors

import (
	"fmt"
	"sort"
)

type actorImpl struct {
	messageChannel chan actorMessage
	path           string
	messageBuffer  []interface{}
	actorImpl      Actor
	context        actorContextImpl
}

type Actor interface {
	OnStart(context ActorContext)
	OnStop()
	Receive(ctxt ActorContext, message interface{})
}

type ActorFactory interface {
	New() Actor
}

func (impl *actorImpl) stop(stopChannel chan<- bool) {
	impl.context.self = nil // self is destructed at this point
	impl.actorImpl.OnStop()

	// Have the control thread unregister the actor
	stopResponseChannel := make(chan interface{})
	defer close(stopResponseChannel)
	impl.context.systemControlChannel <- actorStopRequest{
		path:            impl.path,
		responseChannel: stopResponseChannel,
	}
	<-stopResponseChannel

	// Notify the parent the child is stopped
	if stopChannel != nil {
		stopChannel <- true
	}
}

func (impl *actorImpl) run(responseChannel chan<- ActorRef) {
	ptrToContext := &impl.context
	var stopChannel chan<- bool

	impl.actorImpl.OnStart(ptrToContext)

	// Once the actor is started, notify the creator
	if responseChannel != nil {
		responseChannel <- impl.context.self
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

	impl.stop(stopChannel)
}

func newActor(name string, controlChannel chan<- interface{}, request actorCreateRequest) *actorImpl {
	// Running in the context of the main system goroutine

	var impl = new(actorImpl)
	*impl = actorImpl{
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
			systemControlChannel: controlChannel,
		},
	}

	var actorRef = new(actorRef)
	actorRef.name = name
	actorRef.messageChannel = impl.messageChannel
	impl.context.self = actorRef

	// Owned by the new actor
	go impl.run(request.responseChannel)
	return impl
}
