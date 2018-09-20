package p2p

import (
	"testing"
	"time"

	"github.com/ofgp/ofgp-core/message"
	pb "github.com/ofgp/ofgp-core/proto"
)

func TestCheck(t *testing.T) {
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
		TxID:   "1",
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
	p2pDB.setWaitConfirm("1", &WaitConfirmMsg{
		ScTxId:   "1",
		Opration: confirmed,
		Chain:    message.Bch,
		Info:     nil,
		Time:     time.Now().Unix(),
	})
	// _, noderun := node.RunNew(0, nil)
	// defer noderun.Stop()
	service := newService(checkNode)
	checker := newConfirmChecker(p2pDB, 1, 1, 2, service)
	checker.run()
	time.Sleep(5 * time.Second)
}

func BenchmarkCheck(b *testing.B) {
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
		TxID:   "1",
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
	p2pDB.setWaitConfirm("1", &WaitConfirmMsg{
		ScTxId:   "1",
		Opration: confirmed,
		Chain:    message.Bch,
		Info:     nil,
		Time:     time.Now().Unix(),
	})
	// _, noderun := node.RunNew(0, nil)
	// defer noderun.Stop()
	service := newService(checkNode)
	checker := newConfirmChecker(p2pDB, 1, -1, 200, service)

	for i := 0; i < b.N; i++ {
		checker.check()
	}
}
