package p2p

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

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
		Opration:    0,
		SendAddr:    getBytes(20),
		ReceiveAddr: getBytes(20),
		Chain:       1,
		TokenID:     1,
		Amount:      64,
		Fee:         1,
		ExpiredTime: uint32(time.Now().Unix()),
		SeqID:       getBytes(32),
	}
	data := msg.Encode()
	msg2 := &p2pMsg{}
	msg2.Decode(data)
	t.Logf("data:%d", msg2.Amount)
}

func TestConfirmEncode(t *testing.T) {
	msg := &p2pMsgConfirmed{
		Opration:  2,
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
