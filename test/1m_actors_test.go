// Copyright 2019 Calvin Grunewald. All rights reserved.

package test

import (
	"strconv"
	"testing"

	"github.com/cgrunewald/goactors"
)

func Benchmark1MActors(b *testing.B) {

	for n := 0; n < b.N; n++ {
		system := goactors.NewSystem("test")
		context := system.Context()
		for i := 0; i < 1000000; i++ {
			context.CreateActorFromFunc(func() goactors.Actor {
				return &goactors.DefaultActor{}
			}, ""+strconv.Itoa(i))
		}

		context.Stop(context.SelfRef())
		system.Wait()
	}

}
