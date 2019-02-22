package test

import (
	"sync"
	"testing"

	"github.com/cgrunewald/goactors/actors"
)

type echoActor struct {
	actors.DefaultActor
	t *testing.T
}

func (a *echoActor) Receive(context actors.ActorContext, message interface{}) {
	context.SenderRef().Send(context.SelfRef(), message)
}

type echoActorDoubleSender struct {
	actors.DefaultActor
}

func (a *echoActorDoubleSender) Receive(context actors.ActorContext, message interface{}) {
	context.SenderRef().Send(context.SelfRef(), message)
	context.SenderRef().Send(context.SelfRef(), message)
}

func TestActorAsk(t *testing.T) {
	system := actors.NewSystem("test")
	context := system.Context()

	echoActor := context.CreateActorFromFunc(func() actors.Actor { return &echoActor{t: t} }, "echo")
	future := echoActor.Ask("ping")

	result := future.GetResult()
	resultString := result.(string)
	if resultString != "ping" {
		t.Errorf("Expected %s, received %s", "ping", "resultString")
	}

	result = future.GetResult()
	if result != nil {
		t.Errorf("Expected second call to GetResult to be nil")
	}
}

func TestActorAskDoubleSend(t *testing.T) {
	system := actors.NewSystem("test")
	context := system.Context()

	echoActor := context.CreateActorFromFunc(func() actors.Actor { return &echoActorDoubleSender{} }, "echo")
	future := echoActor.Ask("ping")

	result := future.GetResult()
	resultString := result.(string)
	if resultString != "ping" {
		t.Errorf("Expected %s, received %s", "ping", "resultString")
	}
}

type sendCheckerActor struct {
	actors.DefaultActor
	t  *testing.T
	wg *sync.WaitGroup
}

func (a *sendCheckerActor) Receive(context actors.ActorContext, message interface{}) {
	a.wg.Done()
}

func TestActorAskForward(t *testing.T) {
	system := actors.NewSystem("test")
	context := system.Context()

	wg := sync.WaitGroup{}
	wg.Add(1)

	echoActor := context.CreateActorFromFunc(func() actors.Actor { return &echoActor{} }, "echo")
	sendChecker := context.CreateActorFromFunc(func() actors.Actor { return &sendCheckerActor{t: t, wg: &wg} }, "check")
	future := echoActor.Ask("ping")
	future.ForwardResult(context.SelfRef(), sendChecker)

	wg.Wait()
}
