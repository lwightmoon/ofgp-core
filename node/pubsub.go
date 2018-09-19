package node

import (
	"sync"
)

type pubServer struct {
	sync.RWMutex
	reg      map[string]chan BusinessEvent
	capicity int
}

func newPubServer(capicity int) *pubServer {
	server := &pubServer{
		reg:      make(map[string]chan BusinessEvent),
		capicity: capicity,
	}
	return server
}

func (ps *pubServer) subScribe(business string) chan BusinessEvent {
	ps.Lock()
	defer ps.Unlock()
	if ch, ok := ps.reg[business]; ok {
		return ch
	}
	ch := make(chan BusinessEvent, ps.capicity)
	ps.reg[business] = ch
	return ch
}

func (ps *pubServer) pub(topic string, event BusinessEvent) {
	ps.send(topic, event)
}

func (ps *pubServer) send(topic string, event BusinessEvent) {
	var ch chan BusinessEvent
	ps.RLock()
	ch = ps.reg[topic]
	ps.RUnlock()
	ch <- event
}
