package p2p

import (
	"fmt"
	"testing"
	"time"

	"github.com/ofgp/common/defines"
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
		From:   uint32(defines.CHAIN_CODE_BCH),
		To:     uint32(defines.CHAIN_CODE_ETH),
		Data:   p2pMsg.Encode(),
	}
	p2pInfo := &P2PInfo{
		Event: event,
		Msg:   msgUse,
	}
	p2pDB.setP2PInfo(p2pInfo)
	p2pInfo.Event.TxID = "testTxID2"
	p2pDB.setP2PInfo(p2pInfo)
	p2pInfo.Event.TxID = "testTxID3"
	p2pDB.setP2PInfo(p2pInfo)
	fmt.Printf("p2pInfoIndex:%d\n", p2pInfo.Index)
	info := p2pDB.getP2PInfo("testTxID3")
	fmt.Printf("get p2pInfo txID:%s,Index:%d\n", info.Event.TxID, info.Index)
	// Output: get p2pInfo txID:testTxID3,Index:3
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
		From:   uint32(defines.CHAIN_CODE_BCH),
		To:     uint32(defines.CHAIN_CODE_ETH),
		Data:   p2pMsg.Encode(),
	}
	p2pInfo := &P2PInfo{
		Event: event,
		Msg:   msgUse,
		Index: 0,
	}
	p2pDB.setP2PInfo(p2pInfo)
	// p2pDB.getP2PInfo("testTxID")
	infos := p2pDB.getAllP2PInfos()
	for _, info := range infos {
		t.Logf("info:%s", info.GetScTxID())
	}
}

func TestMatch(t *testing.T) {
	p2pDB.setMatched("a", "b")
	matched := p2pDB.getMatched("a")
	t.Logf("get matched:%s", matched)
	matched = p2pDB.getMatched("b")
	t.Logf("get matched:%s", matched)
}
func TestExistMatch(t *testing.T) {
	key := "testMatch"
	exist := p2pDB.ExistMatched(key)
	t.Logf("exist match:%t", exist)
	p2pDB.setMatchedOne(key, "")
	exist = p2pDB.ExistMatched(key)
	t.Logf("after match:%t", exist)
	t.Logf("get res:%s", p2pDB.getMatched(key))
}

func TestSendedTx(t *testing.T) {
	p2pDB.setSendedInfo(&SendedInfo{
		TxId:     "testTxID",
		SignTerm: 1,
	})
	info := p2pDB.getSendedInfo("testTxID")
	t.Logf("txid:%s,term:%d", info.TxId, info.GetSignTerm())
	p2pDB.delSendedInfo("testTxID")
	info = p2pDB.getSendedInfo("testTxID")
	if info != nil {
		t.Fail()
	}
}

func TestClear(t *testing.T) {
	scTxID := "testTxID2"
	p2pDB.setMatched(scTxID, "b")
	p2pDB.setSendedInfo(&SendedInfo{
		TxId:     scTxID,
		SignTerm: 2,
	})
	p2pDB.clear(scTxID)
	matched := p2pDB.getMatched(scTxID)
	if matched != "" {
		t.Fail()
	}
}

func TestSign(t *testing.T) {
	scTxID := "testTxID2"
	p2pDB.setSendedInfo(&SendedInfo{
		TxId:     scTxID,
		SignTerm: 2,
	})
	exist := p2pDB.existSendedInfo(scTxID + "_")
	t.Logf("exist sended:%v", exist)
}

func TestSetIndex(t *testing.T) {
	p2pDB.setIndex(12)
	res := p2pDB.getIndex()
	t.Logf("get index:%d", res)
}
