package p2p

import (
	"errors"

	"github.com/ofgp/common/defines"
	"github.com/ofgp/ofgp-core/message"
	"github.com/ofgp/ofgp-core/node"
	pb "github.com/ofgp/ofgp-core/proto"
)

type service struct {
	node *node.BraftNode
}

func newService(node *node.BraftNode) *service {
	return &service{
		node: node,
	}
}

func (s *service) makeCreateTxReq(op uint8, info *P2PInfo) (message.CreateReq, error) {
	var addr []byte
	msg := info.GetMsg()
	event := info.GetEvent()
	switch op {
	case confirmed:
		addr = msg.ReceiveAddr
	case back:
		addr = msg.SendAddr
	default:
		p2pLogger.Error("op err")
		return nil, nil
	}
	var req message.CreateReq
	chain := uint8(msg.Chain)
	p2pLogger.Debug("send tx to", "chain", chain)
	switch chain {
	case defines.CHAIN_CODE_BCH:
		fallthrough
	case defines.CHAIN_CODE_BTC:
		req = &node.BaseCreateReq{
			Chain:  msg.Chain,
			ID:     event.GetTxID(),
			Addr:   addr,
			Amount: msg.Amount,
		}
	case defines.CHAIN_CODE_ETH:
		ethReq := &node.EthCreateReq{}
		ethReq.Chain = msg.Chain
		ethReq.ID = event.GetTxID()
		ethReq.Addr = addr
		ethReq.Amount = msg.Amount
		ethReq.TokenTo = msg.TokenId
		req = ethReq
	default:
		p2pLogger.Error("chain type err")
		return nil, errors.New("chain type err")
	}
	return req, nil
}

func (s *service) sendtoSign(req *message.CreateAndSignMsg) {
	signMsg := req.Msg
	p2pLogger.Debug("send to sign ", "scTxID", signMsg.ScTxID, "business", signMsg.Business)
	s.node.CreateAndSign(req)
}

func (s *service) sendTx(data *node.SignedData) error {
	sendReq := &node.SendReq{
		Chain: data.Chain,
		ID:    data.ID,
		Tx:    data.Tx,
	}
	return s.node.SendTx(sendReq)
}

func (s *service) isDone(scTxID string) bool {
	return s.node.IsDone(scTxID)
}

func (s *service) isTxOnChain(txID string, chain uint8) bool {

	return s.node.GetTxByHash(txID, chain) != nil
}

func (s *service) isSignFail(txID string) bool {
	return s.node.IsSignFailed(txID)
}

func (s *service) clear(scTxID string, term int64) {
	s.node.Clear(scTxID, term)
}

func (s *service) accuseWithTerm(term int64) {
	s.node.AccuseWithTerm(term)
}

func (s *service) accuse() {
	s.node.Accuse()
}

func (s *service) markSignFail(scTxID string) {
	s.node.MarkFail(scTxID)
}

func (s *service) commitTx(tx *pb.Transaction) {
	s.node.Commit(tx)
}
