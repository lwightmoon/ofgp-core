package mint

import (
	"github.com/ofgp/ofgp-core/business"
	"github.com/ofgp/ofgp-core/node"
)

type watchedHandler struct {
	business.Handler
}

func (handler *watchedHandler) HandleEvent(event node.BusinessEvent) {
	if val, ok := event.(*node.WatchedEvent); ok {
		watchedEvent := val.GetData()
		mintReq := &MintRequire{}
		mintReq.decode(watchedEvent.GetData())
		
	} else if handler.Successor != nil {
		handler.HandleEvent(event)
	}
}

type signedHandler struct {
	business.Handler
}

func (handler *signedHandler) HandleEvent(event node.BusinessEvent) {

}

type confirmedHandler struct {
	business.Handler
}

func (handler *confirmedHandler) HandleEvent(event node.BusinessEvent) {

}

type commitedHandler struct {
	business.Handler
}

func (handler *commitedHandler) HandleEvent(event node.BusinessEvent) {

}
