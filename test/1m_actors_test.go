package test

import (
	"strconv"
	"testing"

	"github.com/cgrunewald/goactors/actors"
)

func Benchmark1MActors(b *testing.B) {

	for n := 0; n < b.N; n++ {
		system := actors.NewSystem("test")
		context := system.Context()
		for i := 0; i < 1000000; i++ {
			context.CreateActorFromFunc(func() actors.Actor {
				return &actors.DefaultActor{}
			}, ""+strconv.Itoa(i))
		}

		context.Stop(context.SelfRef())
		system.Wait()
	}

}
