package goactors

import (
	"context"
	"errors"
	"time"
)

// ErrFutureTimeout is returned by GetResultWithTimeout when the result is not
// available before the timeout elapses.
var ErrFutureTimeout = errors.New("goactors: timed out waiting for future result")

// Future represents a future result that has yet to be computed
type Future interface {
	// Blocks thread to get current result from the future
	GetResult() interface{}

	// Blocks thread until the result is available or the timeout elapses. Returns
	// ErrFutureTimeout if the timeout elapses first. A timed out future may still
	// be completed later and can be waited on again.
	GetResultWithTimeout(timeout time.Duration) (interface{}, error)

	// Blocks thread until the result is available or the context is done. Returns
	// the context's error if the context is cancelled or its deadline is exceeded
	// before the result arrives.
	GetResultWithContext(ctx context.Context) (interface{}, error)

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

func (future *futureImpl) GetResultWithTimeout(timeout time.Duration) (interface{}, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result, ok := <-future.readChannel:
		if !ok {
			// Future has been handled
			return nil, nil
		}
		return result.message, nil
	case <-timer.C:
		return nil, ErrFutureTimeout
	}
}

func (future *futureImpl) GetResultWithContext(ctx context.Context) (interface{}, error) {
	select {
	case result, ok := <-future.readChannel:
		if !ok {
			// Future has been handled
			return nil, nil
		}
		return result.message, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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
