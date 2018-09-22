package message

import (
	pb "github.com/ofgp/ofgp-core/proto"
)

// WaitSignMsg 待签名数据
type WaitSignMsg struct {
	Business string //业务
	ID       string //标识
	ScTxID   string //源链txid
	Event    *pb.WatchedEvent
	Tx       *pb.NewlyTx //待签名交易
	Recharge []byte
}

// CreateReq 创建交易接口
type CreateReq interface {
	GetChain() uint32
	GetID() string
	GetAddr() []byte
	GetAmount() uint64
}

//CreateAndSignMsg 创建并签名数据
type CreateAndSignMsg struct {
	Req CreateReq
	Msg *WaitSignMsg
}

// GetScTxID 获取源链txid
func (msg *CreateAndSignMsg) GetScTxID() string {
	return msg.Msg.ScTxID
}
