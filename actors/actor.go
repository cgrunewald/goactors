package actors

type Actor interface {
	OnStart(context ActorContext)
	OnStop()
	Receive(sender *ActorRef, message interface{})
}

type ActorFactory interface {
	New() Actor
}
