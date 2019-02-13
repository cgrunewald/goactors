package actors

type ActorContext interface {
	CreateActorFromFactory(factory ActorFactory, name string) *ActorRef
	CreateActorFromFunc(factoryFunc func() Actor, name string) *ActorRef
	FindActor(path string) *ActorRef
	SenderRef() *ActorRef
	ParentRef() *ActorRef
	SelfRef() *ActorRef
	Path() string
	GetChild(name string) *ActorRef
	Stop(ref *ActorRef)
}

type actorContextImpl struct {
	path                 string
	sender               *ActorRef
	parent               *ActorRef
	self                 *ActorRef
	systemControlChannel chan<- interface{}
	children             map[string]*ActorRef
}

func (context *actorContextImpl) CreateActorFromFactory(factory ActorFactory, name string) *ActorRef {
	return context.CreateActorFromFunc(func() Actor { return factory.New() }, name)
}
func (context *actorContextImpl) CreateActorFromFunc(factoryFunc func() Actor, name string) *ActorRef {
	var responseChannel = make(chan *ActorRef)

	defer close(responseChannel)
	context.systemControlChannel <- actorCreateRequest{
		name:            name,
		parent:          context.self,
		factoryFunction: factoryFunc,
		responseChannel: responseChannel,
	}
	var ref = <-responseChannel
	if ref != nil {
		// Context should only be updated on the goroutine owned by this actor
		context.children[name] = ref
	}

	return ref
}

func (context *actorContextImpl) FindActor(path string) *ActorRef {
	var responseChannel = make(chan *ActorRef)

	defer close(responseChannel)
	context.systemControlChannel <- actorLookupRequest{
		name:            path,
		responseChannel: responseChannel,
	}
	var ref = <-responseChannel
	return ref
}

func (context *actorContextImpl) SenderRef() *ActorRef {
	return context.sender
}

func (context *actorContextImpl) SelfRef() *ActorRef {
	return context.self
}

func (context *actorContextImpl) ParentRef() *ActorRef {
	return context.parent
}

func (context *actorContextImpl) Path() string {
	return context.path
}

func (context *actorContextImpl) GetChild(name string) *ActorRef {
	child, ok := context.children[name]
	if ok {
		return child
	}
	return nil
}

func (context *actorContextImpl) Stop(ref *ActorRef) {
	ref.Send(poisonPillMessage{
		resultChannel: nil,
	})
}
