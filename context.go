// Copyright 2019 Calvin Grunewald. All rights reserved.

package goactors

import (
	"sort"
	"sync"
)

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

	// The root actor's context is handed out by ActorSystem.Context() and used from
	// arbitrary goroutines, so access to children is guarded by a mutex
	childrenMutex sync.Mutex
	children      map[string]ActorRef
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
		context.childrenMutex.Lock()
		context.children[request.name] = ref
		context.childrenMutex.Unlock()
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

// sortedChildren returns a snapshot of the actor's children, sorted by name so that
// children are stopped in a consistent order
func (context *actorContextImpl) sortedChildren() []ActorRef {
	context.childrenMutex.Lock()
	defer context.childrenMutex.Unlock()

	names := make([]string, 0, len(context.children))
	for k := range context.children {
		names = append(names, k)
	}
	sort.Strings(names)

	children := make([]ActorRef, 0, len(names))
	for _, name := range names {
		children = append(children, context.children[name])
	}
	return children
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
	context.childrenMutex.Lock()
	defer context.childrenMutex.Unlock()

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
