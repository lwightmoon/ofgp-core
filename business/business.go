package business

import (
	"github.com/ofgp/ofgp-core/message"
	"github.com/ofgp/ofgp-core/node"
	pb "github.com/ofgp/ofgp-core/proto"
)

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

// IService 网关服务interface
type IService interface {
	SendToSign(req *message.CreateAndSignMsg)

	SendTx(data *node.SignedData) error

	IsDone(scTxID string) bool

	IsTxOnChain(txID string, chain uint8) bool

	CommitTx(tx *pb.Transaction)

	SubScribe(business string) chan node.BusinessEvent
}
