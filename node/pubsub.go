package node

import (
	"sync"
)

type pubServer struct {
	sync.RWMutex
	reg      map[uint32]chan BusinessEvent
	capicity int
}

func (ps *pubServer) hasTopic(topic uint32) bool {
	ps.RLock()
	_, ok := ps.reg[topic]
	ps.RUnlock()
	return ok
}

func newPubServer(capicity int) *pubServer {
	server := &pubServer{
		reg:      make(map[uint32]chan BusinessEvent),
		capicity: capicity,
	}
	return server
}

func (ps *pubServer) subScribe(business uint32) chan BusinessEvent {
	ps.Lock()
	defer ps.Unlock()
	if ch, ok := ps.reg[business]; ok {
		return ch
	}
	ch := make(chan BusinessEvent, ps.capicity)
	ps.reg[business] = ch
	return ch
}

func (ps *pubServer) pub(topic uint32, event BusinessEvent) {
	ps.send(topic, event)
}

func (ps *pubServer) send(topic uint32, event BusinessEvent) {
	ps.RLock()
	ch, ok := ps.reg[topic]
	ps.RUnlock()
	if !ok {
		return
	}
	ch <- event
}
