package message

import (
	pb "github.com/ofgp/ofgp-core/proto"
)

const (
	Bch = iota
	Btc
	Eth
)

type WaitSignMsg struct {
	Business string //业务
	ID       string //标识
	ScTxID   string //源链txid
	Event    *pb.WatchedEvent
	Tx       *pb.NewlyTx //待签名交易
}
