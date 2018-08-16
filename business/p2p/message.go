package p2p

import (
	"bytes"
	"encoding/binary"
	"io"
)

const (
	require = iota //event type
	match
	confirmed
	back
)

var bigEndian = binary.BigEndian

func read(r io.Reader, length int) []byte {
	res := make([]byte, length)
	io.ReadFull(r, res)
	return res
}

func readInt(r io.Reader, element interface{}) error {
	var err error
	switch e := element.(type) {
	case *uint8:
		buf := make([]byte, 1)
		_, err = io.ReadFull(r, buf)
		*e = buf[0]
	case *uint32:
		buf := make([]byte, 4)
		_, err = io.ReadFull(r, buf)
		*e = bigEndian.Uint32(buf)
	case *uint64:
		buf := make([]byte, 8)
		_, err = io.ReadFull(r, buf)
		*e = bigEndian.Uint64(buf)
	}
	return err
}

//GetEventtype()==0 监听到
type p2pMsg struct {
	Opration    uint8  //交易操作 0:发起请求 1:匹配请求
	TxID        []byte //交易id 32
	SendAddr    []byte //发送方地址 20
	ReceiveAddr []byte //在对应链接收币的地址 20
	Chain       uint8  //接收方链
	TokenID     uint32 //接收方token
	Amount      uint64 //接收数量
	Fee         uint64 //矿工费
	ExpiredTime uint32 //exchange过期时间
	SeqID       []byte //exchange id 32
}

func (msg *p2pMsg) Decode(data []byte) {
	r := bytes.NewReader(data)
	readInt(r, &msg.Opration)
	txid := read(r, 32)
	msg.TxID = txid
	saddr := read(r, 20)
	msg.SendAddr = saddr
	raddr := read(r, 20)
	msg.ReceiveAddr = raddr

	readInt(r, &msg.Chain)
	readInt(r, &msg.TokenID)
	readInt(r, &msg.Amount)
	readInt(r, &msg.Fee)
	readInt(r, &msg.ExpiredTime)
	seqID := read(r, 32)
	msg.SeqID = seqID
}

//GetEventtype()==1 被确认
type p2pMsgConfirmed struct {
	Opration  uint8  //2 交易被确认 3 交易回退
	ID        []byte //交易标识 32
	TxID      []byte //最终被确认的交易id 32
	Chain     uint8  //所在链
	Confirms  uint8  //确认数
	Height    uint64 //所在区块高度
	BlockHash []byte //所在区块hash 32
	Amount    uint64 //数额
	Fee       uint64 //矿工费
}

func (msg *p2pMsgConfirmed) Decode(data []byte) {
	r := bytes.NewReader(data)
	readInt(r, &msg.Opration)
	id := read(r, 32)
	msg.ID = id
	txid := read(r, 32)
	msg.TxID = txid
	readInt(r, &msg.Chain)
	readInt(r, &msg.Confirms)
	readInt(r, &msg.Height)
	blockHash := read(r, 32)
	msg.BlockHash = blockHash
	readInt(r, &msg.Amount)
	readInt(r, &msg.Fee)
}
