package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cgrunewald/goactors/actors"
)

type PingActor struct {
	actors.DefaultActor
	pongActor actors.ActorRef
	t         *testing.T
}

func (actor *PingActor) Receive(context actors.ActorContext, message interface{}) {
	msg, ok := message.(string)
	if !ok {
		panic("bad message")
	}

	if msg == "Pong" {
		actor.t.Logf("Pong - %v %v\n", context.SelfRef(), context.SenderRef())
		context.SenderRef().Send(context.SelfRef(), "Ping")
	}
}

type PongActor struct {
	actors.DefaultActor
	pingActor actors.ActorRef
	t         *testing.T
	pingCount int8
}

func (actor *PongActor) OnStart(context actors.ActorContext) {
	actor.pingCount = 0

	actor.pingActor = context.FindActor("/test/ping")
	if actor.pingActor == nil {
		panic("Could not find ping actor")
	}

	actor.t.Logf("Ping actor %v, sending Pong message\n", actor.pingActor)
	actor.pingActor.Send(context.SelfRef(), "Pong")
}

func (actor *PongActor) Receive(context actors.ActorContext, message interface{}) {
	msg, ok := message.(string)
	if !ok {
		panic("bad message")
	}

	if msg == "Ping" {
		actor.pingCount++
		if actor.pingCount > 10 {
			context.Stop(context.ParentRef()) // Kill root
		} else {
			actor.t.Logf("Ping - %v %v\n", context.SelfRef(), context.SenderRef())
			context.SenderRef().Send(context.SelfRef(), "Pong")
		}
	}
}

func TestActorSystemPingPongLifecycleTest(t *testing.T) {
	system := actors.NewSystem("test")
	context := system.Context()

	context.CreateActorFromFunc(func() actors.Actor {
		actor := new(PingActor)
		actor.t = t
		return actor
	}, "ping")
	context.CreateActorFromFunc(func() actors.Actor {
		actor := new(PongActor)
		actor.t = t
		return actor
	}, "pong")
	system.Wait()
}

type actor1 struct {
	actors.DefaultActor
	log   *[]string
	mutex *sync.Mutex
	path  string
}

func (a *actor1) OnStart(context actors.ActorContext) {
	a.path = context.Path()
	a.mutex.Lock()
	*a.log = append(*a.log, fmt.Sprintf("Actor1 Start - %s", context.Path()))
	fmt.Println(a.log)
	defer a.mutex.Unlock()
}

func (a *actor1) OnStop() {
	a.mutex.Lock()
	*a.log = append(*a.log, fmt.Sprintf("Actor1 Stop - %s", a.path))
	defer a.mutex.Unlock()
}

func TestActorStartAndShutdownOrder(t *testing.T) {
	messageLog := make([]string, 0, 10)
	mutex := new(sync.Mutex)

	actor1Factory := func() actors.Actor {
		a := new(actor1)
		a.mutex = mutex
		a.log = &messageLog
		return a
	}

	system := actors.NewSystem("test")
	context := system.Context()

	context.CreateActorFromFunc(actor1Factory, "1")
	fmt.Println("created first actor")
	context.CreateActorFromFunc(actor1Factory, "2")
	fmt.Println("created second actor")

	context.Stop(context.SelfRef())
	system.Wait()

	for _, v := range messageLog {
		fmt.Println(v)
	}

	expectedResult := []string{
		"Actor1 Start - /test/1",
		"Actor1 Start - /test/2",
		"Actor1 Stop - /test/1",
		"Actor1 Stop - /test/2",
	}

	if len(expectedResult) != len(messageLog) {
		t.Errorf("logs are not the same size (expected: %v actual: %v)", len(expectedResult), len(messageLog))
		return
	}

	for i, v := range expectedResult {
		if v != messageLog[i] {
			t.Errorf("logs differ at position %d (expected: %v actual %v", i, v, messageLog[i])
		}
	}
}

func TestActorSystemStartedLifecycle(t *testing.T) {
	system := actors.NewSystem("test")
	if system.IsRunning() != true {
		t.Errorf("System %s is not running when it should be running\n", system.Context().Path())
		return
	}

	context := system.Context()
	ref := context.SelfRef()
	if ref == nil {
		t.Error("System's root context does not have a reference to root actor")
		return
	}

	if ref.Path() != "/test" {
		t.Errorf("Unexpected path (expected %s, received %s\n", "/test", ref.Path())
		return
	}

	context.Stop(ref)

	// Need to sleep since Stop is async
	time.Sleep(time.Second)

	if system.IsRunning() != false {
		t.Errorf("System %s is running when it should not be running\n", system.Context().Path())
		return
	}
}
