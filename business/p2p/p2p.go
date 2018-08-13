package p2p

import "github.com/ofgp/ofgp-core/node"

type P2P struct {
	ch chan node.PushEvent
}

func NewP2P(evCh chan node.PushEvent) *P2P {
	return &P2P{
		ch: evCh,
	}
}
func (p2p *P2P) Process() {
	for event := range p2p.ch {
		switch event.(type) {

		}
	}
}
