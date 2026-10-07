// Copyright 2019 Calvin Grunewald. All rights reserved.

package goactors

type actorProxy struct {
	proxiedActor     ActorRef
	messageChannel   chan actorMessage
	bufferedMessages []actorMessage
	stopChannel      chan struct{}
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

	// Kill the proxy go routine if a proxy is being used
	if impl.proxy != nil {
		close(impl.proxy.stopChannel)
		impl.proxy = nil
	}
}

const (
	actorMessageResultStop    = iota
	actorMessageResultTryNext = iota
)

func (impl *actorImpl) tryProcessSystemMessage(message actorMessage) int {
	switch message.message.(type) {
	case poisonPillMessage:
		// fmt.Printf("Received poison pill %v\n", impl.context.path)
		pill := message.message.(poisonPillMessage)
		childrenResultChannel := make(chan bool)
		defer close(childrenResultChannel)

		// Children are stopped one at a time in sorted order for consistent stopping
		for _, child := range impl.context.sortedChildren() {
			child.Send(
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

// runProxy forwards messages from the proxy's unbounded buffer to the proxied actor's
// mailbox. The proxy state is passed in explicitly (rather than read from impl.proxy)
// because it is owned by this goroutine; the actor goroutine only signals shutdown by
// closing proxy.stopChannel.
func runProxy(proxy *actorProxy, target chan<- actorMessage) {
	// Proxy actors don't have an underlying implementation. They don't have a start/stop
	// The don't deal with system messages. They literally forward everything to the
	// underlying actor's message channel.
	if proxy == nil {
		panic("This is not a proxy actor")
	}

loop:
	for {
		if len(proxy.bufferedMessages) > 0 {
			msg := proxy.bufferedMessages[0]
			select {
			case val := <-proxy.messageChannel:
				proxy.bufferedMessages = append(proxy.bufferedMessages, val)
			case target <- msg:
				proxy.bufferedMessages = proxy.bufferedMessages[1:]
			case <-proxy.stopChannel:
				break loop
			}
		} else {
			select {
			case val := <-proxy.messageChannel:
				proxy.bufferedMessages = append(proxy.bufferedMessages, val)
			case <-proxy.stopChannel:
				break loop
			}
		}
	}
}

func (impl *actorImpl) run(responseChannel chan<- ActorRef) {
	ptrToContext := &impl.context

	impl.actorImpl.OnStart(ptrToContext)

	// Once the actor is started, notify the creator
	if responseChannel != nil {
		responseChannel <- impl.context.self
	}

	// fmt.Printf("Actor %s is now receiving messages\n", ptrToContext.path)

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
	behavior := request.factoryFunction()
	if behavior == nil {
		panic("Could not create actor behavior implementation")
	}

	var impl = new(actorImpl)
	*impl = actorImpl{
		path:           name,
		messageChannel: make(chan actorMessage, 10),
		actorImpl:      behavior,

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

	var ref = new(actorRef)
	ref.name = name
	ref.messageChannel = impl.messageChannel
	impl.context.self = ref

	// Create and wire up the proxy if requested. This must happen before the actor's
	// goroutine is started so that the actor (and its creator, via the response
	// channel) only ever observes the proxy ref as its self reference.
	if request.proxy {
		// Create the proxy actor to store message queue and pointer to original actor
		impl.proxy = new(actorProxy)
		impl.proxy.messageChannel = make(chan actorMessage)
		impl.proxy.stopChannel = make(chan struct{})
		impl.proxy.bufferedMessages = make([]actorMessage, 0, 10)
		impl.proxy.proxiedActor = ref

		// Create a new actor ref to the proxy actor for other actors to use. This ensures
		// the proxy's message channel is always used
		proxyRef := new(actorRef)
		proxyRef.name = name
		proxyRef.messageChannel = impl.proxy.messageChannel
		impl.context.self = proxyRef

		go runProxy(impl.proxy, impl.messageChannel)
	}

	// Owned by the new actor
	go impl.run(request.responseChannel)

	return impl
}
