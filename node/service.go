package node

import (
	"github.com/ofgp/ofgp-core/message"
	pb "github.com/ofgp/ofgp-core/proto"
)

func (node *BraftNode) CreateTx(req CreateReq) *pb.NewlyTx {
	tx := node.txInvoker.CreateTx(req)
	return tx
}

func (node *BraftNode) SignTx(msg *message.WaitSignMsg) {
	node.txStore.AddTxtoWaitSign(msg)
}

func (node *BraftNode) SendTx(req ISendReq) {
	node.txInvoker.SendTx(req)
}

func (node *BraftNode) Commit(req *pb.Transaction) {
	node.txStore.CreateInnerTx(req)
}

