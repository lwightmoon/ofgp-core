package p2p

import (
	"github.com/ofgp/ofgp-core/message"
	"github.com/ofgp/ofgp-core/node"
	pb "github.com/ofgp/ofgp-core/proto"
)

type service struct {
	node *node.BraftNode
}

func (s *service) createTx(op uint8, info *P2PInfo) *pb.NewlyTx {
	switch op {
	case confirmed:
		p2pLogger.Debug("create match tx")
	case back:
		p2pLogger.Debug("create back tx")
	}
	return nil
}

func (s *service) sendtoSign(signReq *message.WaitSignMsg) {
	p2pLogger.Debug("send to sign")
}
