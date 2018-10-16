package business

import (
	"github.com/ofgp/ofgp-core/log"

	"github.com/ofgp/ofgp-core/message"
	"github.com/ofgp/ofgp-core/node"
)

var serviceLogger = log.New("DEBUG", "node")

// Service 网关提供的服务
type Service struct {
	node *node.BraftNode
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
