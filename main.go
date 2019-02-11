package main 

import "fmt"

type ActorFactory interface {
	New() *Actor
}

type ActorContext interface {
	CreateActorFromFactory(factory ActorFactory) *ActorRef
	CreateActorFromFunc(factoryFunc func () *Actor) *ActorRef
	Sender() *ActorRef
}

type Actor interface {
	OnStart()
	OnStop()
	Receive(message interface{})
	ID() string
}

type ActorSender interface {
	Tell(message interface{})
}

type ActorRef struct {

}

type ActorMessage struct {
	message interface{}
	sender ActorRef
}

type ActorImpl struct {
	messageChannel chan interface{}
	path string
	messageBuffer []interface{}
}

type ActorSystem struct {
	registry map[string]ActorImpl
	name string
	controlChannel chan interface{}
}
func (self *ActorSystem) CreateActorFromFactory(factory ActorFactory) *ActorRef {
	return self.CreateActorFromFunc(func() *Actor { return factory.New()})
}
func (self *ActorSystem) CreateActorFromFunc(factoryFunc func () *Actor) *ActorRef {
	return nil
}

func Start(system *ActorSystem) {
	fmt.Printf("Starting actor system %s\n", system.name)
	for msg := range system.controlChannel {

	}	
}

func NewActorSystem(name string) *ActorSystem {
	system := new(ActorSystem)
	system.name = name
	system.registry = make(map[string]ActorImpl)
	system.controlChannel = make(chan interface{})
	return system
}

type PingActor struct{

}
func (self *PingActor) OnStart() {}
func (self *PingActor) OnStop() {}
func (self *PingActor) Receive(message interface{}) {}
func (self *PingActor) ID() string { return "name" }

func main() {
	fmt.Println("Hello world")

	var system = NewActorSystem("test")
	var pingActorRef = system.CreateActorFromFunc(func () *Actor { return new (PingActor)});
	var pongActorRef = system.CreateActorFromFunc(func () *Actor { return new (PingActor)});
}
