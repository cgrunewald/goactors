package test

import (
	"testing"
	"time"

	"github.com/cgrunewald/goactors/actors"
)

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
