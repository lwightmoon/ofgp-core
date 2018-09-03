package p2p

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"io"
)

const (
	confirmed = iota //交易操作类型
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
	SendAddr    []byte //发送方地址 20
	ReceiveAddr []byte //在对应链接收币的地址 20
	Chain       uint8  //接收方链
	TokenID     uint32 //接收方token
	Amount      uint64 //接收数量
	Fee         uint64 //矿工费
	ExpiredTime uint32 //exchange过期时间
	RequireAddr []byte //要发送到的地址
}

func (msg *p2pMsg) toPBMsg() *P2PMsg {
	return &P2PMsg{
		SendAddr:    hex.EncodeToString(msg.SendAddr),
		ReceiveAddr: hex.EncodeToString(msg.ReceiveAddr),
		Chain:       uint32(msg.Chain),
		TokenId:     uint32(msg.TokenID),
		Amount:      msg.Amount,
		Fee:         msg.Fee,
		ExpiredTime: msg.ExpiredTime,
		RequireAddr: hex.EncodeToString(msg.RequireAddr),
	}
}

func (msg *p2pMsg) Decode(data []byte) {
	r := bytes.NewReader(data)
	saddr := read(r, 20)
	msg.SendAddr = saddr
	raddr := read(r, 20)
	msg.ReceiveAddr = raddr

	readInt(r, &msg.Chain)
	readInt(r, &msg.TokenID)
	readInt(r, &msg.Amount)
	readInt(r, &msg.Fee)
	readInt(r, &msg.ExpiredTime)
	requireAddr := read(r, 20)
	msg.RequireAddr = requireAddr
}

func (msg *p2pMsg) Encode() []byte {
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, msg.SendAddr)
	binary.Write(buf, binary.BigEndian, msg.ReceiveAddr)
	binary.Write(buf, binary.BigEndian, msg.Chain)
	binary.Write(buf, binary.BigEndian, msg.TokenID)
	binary.Write(buf, binary.BigEndian, msg.Amount)
	binary.Write(buf, binary.BigEndian, msg.Fee)
	binary.Write(buf, binary.BigEndian, msg.ExpiredTime)
	binary.Write(buf, binary.BigEndian, msg.RequireAddr)
	return buf.Bytes()
}

//GetEventtype()==1 被确认
type p2pMsgConfirmed struct {
	Opration  uint8  //0 交易被确认 1 交易回退
	ID        []byte //交易标识 32 对应watchedEvent的txID
	Chain     uint8  //所在链
	Confirms  uint8  //确认数
	Height    uint64 //所在区块高度
	BlockHash []byte //所在区块hash 32
	Amount    uint64 //数额
	Fee       uint64 //矿工费
}

// toPBMsg 转为pb数据结构 序列化
func (msg *p2pMsgConfirmed) toPBMsg() *P2PConfirmMsg {
	return &P2PConfirmMsg{
		Opration:  uint32(msg.Opration),
		Id:        hex.EncodeToString(msg.ID),
		Chain:     uint32(msg.Chain),
		Height:    msg.Height,
		BlockHash: hex.EncodeToString(msg.BlockHash),
		Amount:    msg.Amount,
		Fee:       msg.Fee,
	}
}

func (msg *p2pMsgConfirmed) Decode(data []byte) {
	r := bytes.NewReader(data)
	readInt(r, &msg.Opration)
	id := read(r, 32)
	msg.ID = id
	readInt(r, &msg.Chain)
	readInt(r, &msg.Confirms)
	readInt(r, &msg.Height)
	blockHash := read(r, 32)
	msg.BlockHash = blockHash
	readInt(r, &msg.Amount)
	readInt(r, &msg.Fee)
}

func (msg *p2pMsgConfirmed) Encode() []byte {
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, msg.Opration)
	binary.Write(buf, binary.BigEndian, msg.ID)
	binary.Write(buf, binary.BigEndian, msg.Chain)
	binary.Write(buf, binary.BigEndian, msg.Confirms)
	binary.Write(buf, binary.BigEndian, msg.Height)
	binary.Write(buf, binary.BigEndian, msg.BlockHash)
	binary.Write(buf, binary.BigEndian, msg.Amount)
	binary.Write(buf, binary.BigEndian, msg.Fee)
	return buf.Bytes()
}
