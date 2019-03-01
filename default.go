// Copyright 2019 Calvin Grunewald. All rights reserved.

package goactors

type DefaultActor struct{}

func (root *DefaultActor) OnStart(ctxt ActorContext) {
}
func (root *DefaultActor) OnStop() {
}
func (root *DefaultActor) Receive(ctt ActorContext, message interface{}) {
}
