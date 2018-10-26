package p2p

import (
	"errors"
	ew "swap/ethwatcher"

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

func makeCreateTxReq(op uint8, info *P2PInfo) (message.CreateReq, error) {
	var addr []byte
	var chain uint8
	var token uint32
	msg := info.GetMsg()
	event := info.GetEvent()
	switch op {
	case confirmed:
		addr = msg.ReceiveAddr
		chain = uint8(msg.Chain)
		token = msg.TokenId
	case back:
		addr = msg.SendAddr
		chain = uint8(info.Event.GetFrom())
		token = msg.FromToken
	default:
		p2pLogger.Error("op err", "scTxID", info.Event.GetTxID())
		return nil, nil
	}
	var req message.CreateReq
	p2pLogger.Debug("send tx to", "chain", chain)
	switch chain {
	case defines.CHAIN_CODE_BCH:
		fallthrough
	case defines.CHAIN_CODE_BTC:
		req = &node.BaseCreateReq{
			Business: event.GetBusiness(),
			Chain:    uint32(chain),
			ID:       event.GetTxID(),
			Addr:     addr,
			Amount:   msg.Amount,
			From:     uint8(event.GetFrom()),
		}
	case defines.CHAIN_CODE_ETH:
		ethReq := &node.EthCreateReq{}
		ethReq.Business = event.GetBusiness()
		ethReq.Chain = uint32(chain)
		ethReq.ID = event.GetTxID()
		ethReq.Addr = addr
		ethReq.Amount = msg.Amount
		ethReq.Method = ew.VOTE_METHOD_MATCHSWAP
		ethReq.TokenTo = token
		ethReq.From = uint8(event.GetFrom())
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
