package actors

import (
	"fmt"
	"sort"
)

type actorProxy struct {
	proxiedActor     ActorRef
	messageChannel   chan<- actorMessage
	bufferedMessages []actorMessage
}

type actorImpl struct {
	messageChannel chan actorMessage
	path           string
	messageBuffer  []interface{}
	actorImpl      Actor
	context        actorContextImpl
	proxy          *actorProxy
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

const (
	actorMessageResultStop    = iota
	actorMessageResultTryNext = iota
)

func (impl *actorImpl) tryProcessSystemMessage(message actorMessage) int {
	switch message.message.(type) {
	case poisonPillMessage:
		fmt.Printf("Received poison pill %v\n", impl.context.path)
		pill := message.message.(poisonPillMessage)
		childrenResultChannel := make(chan bool)
		defer close(childrenResultChannel)

		// Sort the children so we have consistent stopping
		sortedChildren := make([]string, 0, len(impl.context.children))
		for k := range impl.context.children {
			sortedChildren = append(sortedChildren, k)
		}

		sort.Strings(sortedChildren)
		for _, val := range sortedChildren {
			impl.context.children[val].Send(
				impl.context.SelfRef(),
				poisonPillMessage{resultChannel: childrenResultChannel})
			<-childrenResultChannel
		}

		impl.stop(pill.resultChannel)
		return actorMessageResultStop
	default:
		return actorMessageResultTryNext
	}

}

func (impl *actorImpl) run(responseChannel chan<- ActorRef) {
	ptrToContext := &impl.context

	impl.actorImpl.OnStart(ptrToContext)

	// Once the actor is started, notify the creator
	if responseChannel != nil {
		responseChannel <- impl.context.self
	}

	fmt.Printf("Actor %s is now receiving messages\n", ptrToContext.path)

loop:
	for actorMsg := range impl.messageChannel {
		ptrToContext.sender = actorMsg.sender

		if systemProcessResult := impl.tryProcessSystemMessage(actorMsg); systemProcessResult == actorMessageResultStop {
			// the actor system is shut down at this point, so just kill the loop
			break loop
		} else if systemProcessResult == actorMessageResultTryNext {
			impl.actorImpl.Receive(ptrToContext, actorMsg.message)
		}
		ptrToContext.sender = nil
	}
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
