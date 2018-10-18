package business

import (
	"github.com/ofgp/ofgp-core/log"

	"github.com/ofgp/ofgp-core/message"
	"github.com/ofgp/ofgp-core/node"
	pb "github.com/ofgp/ofgp-core/proto"
)

var serviceLogger = log.New("DEBUG", "node")

// Service 网关提供的服务
type Service struct {
	node *node.BraftNode
}

// NewService create
func NewService(node *node.BraftNode) *Service {
	return &Service{
		node: node,
	}
}

// SendToSign send to sign
func (s *Service) SendToSign(req *message.CreateAndSignMsg) {
	signMsg := req.Msg
	serviceLogger.Debug("send to sign", "scTxID", signMsg.ScTxID, "business", signMsg.Business)
	s.node.CreateAndSign(req)
}

// SendTx 发送交易
func (s *Service) SendTx(data *node.SignedData) error {
	sendReq := &node.SendReq{
		Chain: data.Chain,
		ID:    data.ID,
		Tx:    data.Tx,
	}
	return s.node.SendTx(sendReq)
}

// IsDone tx 是否已完成
func (s *Service) IsDone(scTxID string) bool {
	return s.node.IsDone(scTxID)
}

// IsTxOnChain tx 是否在链上
func (s *Service) IsTxOnChain(txID string, chain uint8) bool {
	return s.node.GetTxByHash(txID, chain) != nil
}

// IsSignFail check是否签名失败
func (s *Service) IsSignFail(txID string) bool {
	return s.node.IsSignFailed(txID)
}

// CommitTx commit tx
func (s *Service) CommitTx(tx *pb.Transaction) {
	s.node.Commit(tx)
}

// SubScribe 订阅相关业务的Event
func (s *Service) SubScribe(business string) chan node.BusinessEvent {
	ch := s.node.SubScribe(business)
	return ch
}
