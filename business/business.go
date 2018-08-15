package business

import "github.com/ofgp/ofgp-core/node"

type IHandler interface {
	SetSuccessor(IHandler)
	HandleEvent(event node.BusinessEvent)
}

type Handler struct {
	Successor IHandler
}

func (h *Handler) SetSuccessor(i IHandler) {
	if h == nil {
		return
	}
	h.Successor = i
}

type Message interface {
	Decode([]byte)
}
