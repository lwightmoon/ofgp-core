package p2p

import (
	"bytes"
	"encoding/binary"
	"testing"
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
	t.Logf("fee:%v,txID:%v", msg.Fee, msg.TxID)
}
