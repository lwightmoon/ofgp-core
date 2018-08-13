package p2p

import "github.com/ofgp/ofgp-core/node"

type P2P struct {
	ch   chan node.BusinessEvent
	node *node.BraftNode
}

const business = "p2p"

func NewP2P(node *node.BraftNode) *P2P {
	p2p := &P2P{
		node: node,
	}
	//向node订阅业务相关事件
	ch := node.SubScribe(business)
	p2p.ch = ch
	return p2p
}

func (p2p *P2P) processEvent() {
	for event := range p2p.ch {
		switch event.(type) {
		case *node.WatchedEvent: //监听到交易

		case *node.SignedEvent: //被签名

		case *node.ConfirmEvent: //被确认

		case *node.CommitedEvent: //交易数据被网关提交

		}
	}
}
