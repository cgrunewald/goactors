package actors

type ActorRef struct {
	name           string
	messageChannel chan<- actorMessage
}

func (self *ActorRef) Name() string {
	return self.name
}

func (self *ActorRef) Send(message interface{}) {
	self.messageChannel <- actorMessage{sender: self, message: message}
}
