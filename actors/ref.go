package actors

type ActorRef interface {
	Path() string
	Send(message interface{}) error
}

type actorRef struct {
	name           string
	messageChannel chan<- actorMessage
}

func (self *actorRef) Path() string {
	return self.name
}

func (self *actorRef) Send(message interface{}) error {
	self.messageChannel <- actorMessage{sender: self, message: message}
	return nil // TOOD - return error if channel is closed
}
