package main

import (
	"bufio"
	"fmt"
	"os"

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
func (self *PingActor) Receive(sender *actors.ActorRef, message interface{}) {
	val, ok := message.(string)
	if ok {
		if val == "Pong" {
			sender.Send("Ping")
		} else if val == "Ping" {
			sender.Send("Pong")
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
	var pingActorRef = system.CreateActorFromFunc(newPingPongActor("Ping"), "Ping")
	var pongActorRef = system.CreateActorFromFunc(newPingPongActor("Pong"), "Pong")

	fmt.Println(pingActorRef.Name(), pongActorRef.Name())
	fmt.Println("starting pong")
	pingActorRef.Send("Pong")

	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
}
