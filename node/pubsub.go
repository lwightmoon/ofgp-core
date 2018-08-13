package node

type pubServer struct {
	reg      map[string]chan PushEvent
	capicity int
}

func newPubServer(capicity int) *pubServer {
	server := &pubServer{
		reg:      make(map[string]chan PushEvent),
		capicity: capicity,
	}
	return server
}

func (ps *pubServer) subScribe(business string) chan PushEvent {
	if ch, ok := ps.reg[business]; ok {
		return ch
	}
	ch := make(chan PushEvent, ps.capicity)
	ps.reg[business] = ch
	return ch
}

func (ps *pubServer) pub(topic string, event PushEvent) {
	ps.send(topic, event)
}

func (ps *pubServer) send(topic string, event PushEvent) {
	ch := ps.reg[topic]
	ch <- event
}
