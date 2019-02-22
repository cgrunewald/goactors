package actors

type ActorRef interface {
	Path() string
	Send(sender ActorRef, message interface{})
	Ask(message interface{}) Future
}

type actorRef struct {
	name           string
	messageChannel chan<- actorMessage
}

func (self *actorRef) Path() string {
	return self.name
}

func (self *actorRef) Send(sender ActorRef, message interface{}) {
	self.messageChannel <- actorMessage{sender: sender, message: message}
}

func (ref *actorRef) Ask(message interface{}) Future {
	future := newFuture()
	ref.messageChannel <- actorMessage{sender: future, message: message}
	return future
}
