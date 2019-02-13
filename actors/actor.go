package actors

type Actor interface {
	OnStart(context ActorContext)
	OnStop()
	Receive(ctxt ActorContext, message interface{})
}

type ActorFactory interface {
	New() Actor
}
