package test

import (
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
