package p2p

import (
	"bytes"
	"fmt"
	"testing"

	pb "github.com/ofgp/ofgp-core/proto"
)

func TestP2PTx(t *testing.T) {
	p2pInfo1 := &P2PInfo{
		Event: &pb.WatchedEvent{
			Business: "p2p",
			TxID:     "inof1",
		},
		Msg: &P2PMsg{
			SendAddr:    "sendarr1",
			ReceiveAddr: "receive1",
		},
	}
	p2pInfo2 := &P2PInfo{
		Event: &pb.WatchedEvent{
			Business: "p2p",
			TxID:     "inof2",
		},
		Msg: &P2PMsg{
			SendAddr:    "sendarr2",
			ReceiveAddr: "receive2",
		},
	}

	tx := &P2PTx{
		SeqId:     []byte("testSeqID"),
		Initiator: p2pInfo1,
		Matcher:   p2pInfo2,
	}
	p2pDB.setP2PTx(tx, tx.SeqId)
	id := tx.SeqId
	res := p2pDB.getP2PTx(id)
	if res.Initiator.Event.TxID != "inof1" {
		fmt.Println(res.Initiator.Event.TxID)
		t.Error("tx set get err")
	}
}

func TestID(t *testing.T) {
	seqID := []byte("testSeqID")
	p2pDB.setTxSeqIDMap("txIDtest", seqID)
	queryID := p2pDB.getTxSeqID("txIDtest")
	if !bytes.Equal(seqID, queryID) {
		t.Error("get seqID err")
	}
}
