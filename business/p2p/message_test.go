package p2p

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/ofgp/ofgp-core/message"
)

// watcher require
type P2pRequire struct {
	SendAddr    []byte //发送方地址 20
	ReceiveAddr []byte //在对应链接收币的地址 20
	ChainId     uint8  //接收方链
	TokenId     uint32 //接收方token
	Amount      uint64 //接收数量
	Fee         uint64 //矿工费
	Expire      uint32 //exchange过期时间
	RequireAddr []byte //要发送到的地址
}

func (pr P2pRequire) Encode() []byte {
	buf := make([]byte, 85)
	off := copy(buf[0:], pr.SendAddr)

	off += copy(buf[off:], pr.ReceiveAddr)

	off += copy(buf[off:], []byte{pr.ChainId})

	binary.BigEndian.PutUint32(buf[off:], pr.TokenId)
	off += 4

	binary.BigEndian.PutUint64(buf[off:], pr.Amount)
	off += 8

	binary.BigEndian.PutUint64(buf[off:], pr.Fee)
	off += 8

	binary.BigEndian.PutUint32(buf[off:], pr.Expire)
	off += 4

	copy(buf[off:], pr.RequireAddr)
	fmt.Printf("encode bytes is %v \n", hex.EncodeToString(buf))

	return buf
}

//watcher confirm
type P2pConfirm struct {
	ShouldMatched []byte //交易标识 32 对应watchedEvent的txID
	ChainId       uint8  //所在链
	Confirms      uint64 //确认数
	BlockHeight   uint64 //所在区块高度
	BlockHash     []byte //所在区块hash 32
	Amount        uint64 //数额
	Fee           uint64 //矿工费
}

func (pc P2pConfirm) Encode() []byte {
	buf := make([]byte, 97)
	off := copy(buf[0:], pc.ShouldMatched)

	off += copy(buf[off:], []byte{pc.ChainId})

	binary.BigEndian.PutUint64(buf[off:], pc.Confirms)
	off += 8

	binary.BigEndian.PutUint64(buf[off:], pc.BlockHeight)
	off += 8

	off += copy(buf[off:], pc.BlockHash)

	binary.BigEndian.PutUint64(buf[off:], pc.Amount)
	off += 8

	binary.BigEndian.PutUint64(buf[off:], pc.Fee)
	off += 8

	fmt.Printf("encode bytes is %v \n", hex.EncodeToString(buf))

	return buf
}

func TestWatcherMsg(t *testing.T) {
	watcherMsg := &P2pRequire{
		SendAddr:    getBytes(20),
		ReceiveAddr: getBytes(20),
		ChainId:     message.Bch,
		TokenId:     1,
		Amount:      90,
		Fee:         1,
		Expire:      uint32(time.Now().Unix()),
		RequireAddr: getBytes(20),
	}
	msg := &p2pMsg{}
	msg.Decode(watcherMsg.Encode())
	t.Logf("decode amount:%d", watcherMsg.Amount)
}
func TestWatcherConfirm(t *testing.T) {
	confirm := &P2pConfirm{
		ShouldMatched: getBytes(32),
		ChainId:       message.Eth,
		Confirms:      2,
		BlockHeight:   2,
		BlockHash:     getBytes(32),
		Amount:        64,
		Fee:           20,
	}
	data := confirm.Encode()
	msg := &p2pMsgConfirmed{}
	msg.Decode(data)
	t.Logf("confirm data:%d", msg.Amount)
}
func TestDecode(t *testing.T) {
	data := make([]byte, 32)
	for i := 0; i < 32; i++ {
		data[i] = byte(i)
	}
	data1 := make([]byte, 20)
	for i := 0; i < 20; i++ {
		data1[i] = byte(i)
	}
	var bb bytes.Buffer
	buf := bytes.NewBuffer(bb.Bytes())
	var op, chain uint8
	op = 10
	chain = 8
	binary.Write(buf, binary.BigEndian, &op)
	buf.Write(data)
	buf.Write(data1)
	buf.Write(data1)
	binary.Write(buf, binary.BigEndian, &chain)
	var tokenID uint32
	tokenID = 100
	binary.Write(buf, binary.BigEndian, &tokenID)
	var amount, fee uint64
	amount = 1024
	fee = 512
	binary.Write(buf, binary.BigEndian, &amount)
	binary.Write(buf, binary.BigEndian, &fee)
	var et uint32
	et = 2048
	binary.Write(buf, binary.BigEndian, &et)
	buf.Write(data)
	msg := &p2pMsg{}
	msg.Decode(buf.Bytes())
	t.Logf("fee:%v", msg.Fee)
}

func getBytes(size int) []byte {
	res := make([]byte, size)
	for i := 0; i < size; i++ {
		res[i] = byte(i)
	}
	return res
}
func TestEncode(t *testing.T) {
	msg := &p2pMsg{
		SendAddr:    getBytes(20),
		ReceiveAddr: getBytes(20),
		Chain:       1,
		TokenID:     1,
		Amount:      64,
		Fee:         1,
		ExpiredTime: uint32(time.Now().Unix()),
		RequireAddr: getBytes(32),
	}
	data := msg.Encode()
	msg2 := &p2pMsg{}
	msg2.Decode(data)
	t.Logf("data:%d", msg2.Amount)
}

func TestConfirmEncode(t *testing.T) {
	msg := &p2pMsgConfirmed{
		ID:        getBytes(32),
		Chain:     1,
		Confirms:  7,
		Height:    10,
		BlockHash: getBytes(32),
		Amount:    1024,
		Fee:       1,
	}
	data := msg.Encode()
	msg1 := &p2pMsgConfirmed{}
	msg1.Decode(data)
	t.Logf("data:%d", msg1.Amount)
}
