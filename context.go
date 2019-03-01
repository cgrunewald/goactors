// Copyright 2019 Calvin Grunewald. All rights reserved.

package goactors

type ActorContext interface {
	CreateActorFromFunc(factoryFunc func() Actor, name string) ActorRef
	CreateProxyActorFromFunc(factoryFunc func() Actor, name string) ActorRef
	FindActor(path string) ActorRef
	SenderRef() ActorRef
	ParentRef() ActorRef
	SelfRef() ActorRef
	Path() string
	GetChild(name string) ActorRef
	Stop(ref ActorRef)
}

type actorContextImpl struct {
	path                 string
	sender               ActorRef
	parent               ActorRef
	self                 ActorRef
	systemControlChannel chan<- interface{}
	children             map[string]ActorRef
}

func (context *actorContextImpl) CreateActorFromFunc(factoryFunc func() Actor, name string) ActorRef {
	return context.createActor(actorCreateRequest{
		name:            name,
		parent:          context.self,
		factoryFunction: factoryFunc,
	})
}

func (context *actorContextImpl) CreateProxyActorFromFunc(factoryFunc func() Actor, name string) ActorRef {
	return context.createActor(actorCreateRequest{
		name:            name,
		parent:          context.self,
		factoryFunction: factoryFunc,
		proxy:           true,
	})
}

func (context *actorContextImpl) createActor(request actorCreateRequest) ActorRef {
	responseChannel := make(chan ActorRef)
	request.responseChannel = responseChannel

	defer close(responseChannel)
	context.systemControlChannel <- request
	var ref = <-responseChannel
	if ref != nil {
		// Context should only be updated on the goroutine owned by this actor
		context.children[request.name] = ref
	}

	return ref
}

func (context *actorContextImpl) FindActor(path string) ActorRef {
	var responseChannel = make(chan ActorRef)

	defer close(responseChannel)
	context.systemControlChannel <- actorLookupRequest{
		name:            path,
		responseChannel: responseChannel,
	}
	var ref = <-responseChannel
	return ref
}

func (context *actorContextImpl) SenderRef() ActorRef {
	return context.sender
}

func (context *actorContextImpl) SelfRef() ActorRef {
	return context.self
}

func (context *actorContextImpl) ParentRef() ActorRef {
	return context.parent
}

func (context *actorContextImpl) Path() string {
	return context.path
}

func (context *actorContextImpl) GetChild(name string) ActorRef {
	child, ok := context.children[name]
	if ok {
		return child
	}
	return nil
}

func (context *actorContextImpl) Stop(ref ActorRef) {
	ref.Send(context.SelfRef(), poisonPillMessage{
		resultChannel: nil,
	})
}
