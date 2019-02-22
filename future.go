package goactors

// Future represents a future result that has yet to be computed
type Future interface {
	// Blocks thread to get current result from the future
	GetResult() interface{}

	// Forwards the future's reslult to the specified actor. This is a non-blocking call
	ForwardResult(sender ActorRef, target ActorRef)
}

type futureImpl struct {
	writeChannel chan<- actorMessage
	readChannel  <-chan actorMessage
}

func newFuture() *futureImpl {
	c := make(chan actorMessage, 1)
	return &futureImpl{
		writeChannel: c,
		readChannel:  c,
	}
}

func (future *futureImpl) Path() string {
	return "future"
}

func (future *futureImpl) Send(sender ActorRef, message interface{}) {
	if future.writeChannel == nil {
		return
	}

	future.writeChannel <- actorMessage{sender: sender, message: message}
	close(future.writeChannel)
	future.writeChannel = nil
}

func (future *futureImpl) Ask(message interface{}) Future {
	panic("Should not `Ask` on a future")
}

func (future *futureImpl) GetResult() interface{} {
	result, ok := <-future.readChannel
	if !ok {
		// Future has been handled
		return nil
	}

	return result.message
}

func (future *futureImpl) doForwardResult(sender ActorRef, target ActorRef) {
	result := future.GetResult()
	if result == nil {
		return
	}

	target.Send(sender, result)
}

func (future *futureImpl) ForwardResult(sender ActorRef, target ActorRef) {
	go future.doForwardResult(sender, target)
}
