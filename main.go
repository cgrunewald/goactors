package main

import (
	"fmt"

	"github.com/cgrunewald/goactors/actors"
)

type PingActor struct {
}

func (self *PingActor) OnStart(ctxt actors.ActorContext) {
	fmt.Println("Starting ping actor", ctxt.ParentRef())
}
func (self *PingActor) OnStop() {
	fmt.Println("Stopping actor")
}
func (self *PingActor) Receive(ctxt actors.ActorContext, message interface{}) {
	val, ok := message.(string)
	if ok {
		if val == "Pong" {
			ctxt.SenderRef().Send(ctxt.SelfRef(), "Ping")
		} else if val == "Ping" {
			ctxt.SenderRef().Send(ctxt.SelfRef(), "Pong")
		} else {
			panic("what the fuck")
		}
	}
}

func newPingPongActor(t string) func() actors.Actor {
	return func() actors.Actor {
		return new(PingActor)
	}
}

func main() {
	fmt.Println("Hello world")

	var system = actors.NewSystem("test")
	var context = system.Context()
	var pingActorRef = context.CreateActorFromFunc(newPingPongActor("Ping"), "Ping")
	var pongActorRef = context.CreateActorFromFunc(newPingPongActor("Pong"), "Pong")

	fmt.Println(pingActorRef.Path(), pongActorRef.Path())
	fmt.Println("starting pong")
	pingActorRef.Send(pongActorRef, "Pong")

	system.Wait()
	//reader := bufio.NewReader(os.Stdin)
	//reader.ReadString('\n')
}
