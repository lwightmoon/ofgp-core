package p2p

import (
	"fmt"

	"github.com/ofgp/ofgp-core/log"

	"github.com/ofgp/ofgp-core/business"
	"github.com/ofgp/ofgp-core/node"
	"github.com/spf13/viper"
)

var p2pLogger = log.New(viper.GetString("loglevel"), "node")

type P2P struct {
	ch      chan node.BusinessEvent
	node    *node.BraftNode
	handler business.IHandler
}

const businessName = "p2p"

func NewP2P(node *node.BraftNode) *P2P {
	p2p := &P2P{
		node: node,
	}
	//向node订阅业务相关事件
	ch := node.SubScribe(businessName)
	p2p.ch = ch

	//初始化处理链
	wh := new(watchedHandler)
	sh := new(sigenedHandler)
	confirmH := new(confirmHandler)
	commitH := new(commitHandler)
	wh.SetSuccessor(sh)
	sh.SetSuccessor(confirmH)
	confirmH.SetSuccessor(commitH)
	p2p.handler = wh
	return p2p
}

func (p2p *P2P) processEvent() {

	for event := range p2p.ch {
		p2p.handler.HandleEvent(event)
	}
}

type watchedHandler struct {
	business.Handler
}

func (wh *watchedHandler) HandleEvent(event node.BusinessEvent) {
	if _, ok := event.(*node.WatchedEvent); ok {
		p2pLogger.Info("handle watched")
	} else if wh.Successor != nil {
		wh.Successor.HandleEvent(event)
	}
}

type sigenedHandler struct {
	business.Handler
}

func (sh *sigenedHandler) HandleEvent(event node.BusinessEvent) {
	if _, ok := event.(*node.SignedEvent); ok {
		p2pLogger.Info("handle signed")
	} else if sh.Successor != nil {
		sh.Successor.HandleEvent(event)
	}
}

type confirmHandler struct {
	business.Handler
}

func (ch *confirmHandler) HandleEvent(event node.BusinessEvent) {
	if _, ok := event.(*node.ConfirmEvent); ok {
		p2pLogger.Info("handle confirm")
	} else {
		p2pLogger.Error("could not handle event", "event", event.GetData())
	}
}

type commitHandler struct {
	business.Handler
}

func (ch *commitHandler) HandleEvent(event node.BusinessEvent) {
	if _, ok := event.(*node.CommitedEvent); ok {
		fmt.Println("handle Commited")
	} else if ch.Successor != nil {
		ch.Successor.HandleEvent(event)
	}
}
