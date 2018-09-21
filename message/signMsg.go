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
