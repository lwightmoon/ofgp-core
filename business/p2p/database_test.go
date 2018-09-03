package p2p

import (
	"fmt"
	"testing"
	"time"

	"github.com/ofgp/ofgp-core/message"
	pb "github.com/ofgp/ofgp-core/proto"
)

func ExampleP2PInfo() {
	requireAddr := getBytes(20)
	p2pMsg := &p2pMsg{
		SendAddr:    getBytes(20),
		ReceiveAddr: getBytes(20),
		Chain:       1,
		TokenID:     1,
		Amount:      64,
		Fee:         1,
		ExpiredTime: uint32(time.Now().Unix()),
		RequireAddr: requireAddr,
	}
	msgUse := p2pMsg.toPBMsg()
	event := &pb.WatchedEvent{
		TxID:   "testTxID",
		Amount: 1,
		From:   message.Bch,
		To:     message.Eth,
		Data:   p2pMsg.Encode(),
	}
	p2pInfo := &P2PInfo{
		Event: event,
		Msg:   msgUse,
	}
	p2pDB.setP2PInfo(p2pInfo)
	info := p2pDB.getP2PInfo(event.TxID)
	fmt.Printf("get p2pInfo txID:%s\n", info.Event.TxID)
	// Output: get p2pInfo txID:testTxID
}

func TestWaitConfirm(t *testing.T) {
	p2pDB.setWaitConfirm("init", &WaitConfirmMsg{
		Opration: confirmed,
	})
	res := p2pDB.getWaitConfirm("init")
	fmt.Printf("operation:%d,info:%t", res.GetOpration(), res.Info == nil)
}

func TestGetAllInfo(t *testing.T) {
	requireAddr := getBytes(20)
	p2pMsg := &p2pMsg{
		SendAddr:    getBytes(20),
		ReceiveAddr: getBytes(20),
		Chain:       1,
		TokenID:     1,
		Amount:      64,
		Fee:         1,
		ExpiredTime: uint32(time.Now().Unix()),
		RequireAddr: requireAddr,
	}
	msgUse := p2pMsg.toPBMsg()
	event := &pb.WatchedEvent{
		TxID:   "testTxID",
		Amount: 1,
		From:   message.Bch,
		To:     message.Eth,
		Data:   p2pMsg.Encode(),
	}
	p2pInfo := &P2PInfo{
		Event: event,
		Msg:   msgUse,
	}
	p2pDB.setP2PInfo(p2pInfo)
	infos := p2pDB.getAllP2PInfos()
	for _, info := range infos {
		t.Logf("info:%s", info.GetScTxID())
	}
}

func TestMatch(t *testing.T) {
	p2pDB.setMatched("a", "b")
	matched := p2pDB.getMatched("a")
	t.Logf("get matched:%s", matched)
}

func TestSendedTx(t *testing.T) {
	p2pDB.setSendedTx(&SendedTx{
		TxId:     "testTxID",
		Time:     time.Now().Unix(),
		SignTerm: 1,
	})
	p2pDB.setSendedTx(&SendedTx{
		TxId:     "testTxID2",
		Time:     time.Now().Unix(),
		SignTerm: 2,
	})
	p2pDB.delSendedTx("testTxID2")
	txs := p2pDB.getAllSendedTx()
	for _, tx := range txs {
		t.Logf("txid:%s", tx.TxId)
	}
}
