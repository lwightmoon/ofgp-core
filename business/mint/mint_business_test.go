package mint

import (
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"

	proto "github.com/golang/protobuf/proto"
	"github.com/ofgp/common/defines"
	"github.com/ofgp/ofgp-core/business"
	"github.com/ofgp/ofgp-core/node"
	pb "github.com/ofgp/ofgp-core/proto"
)

func TestGetRecharge(t *testing.T) {
	event := &pb.WatchedEvent{
		Business:  "mint",
		EventType: 1,
		TxID:      "testTxID",
		Amount:    1,
		From:      uint32(defines.CHAIN_CODE_BCH),
		To:        uint32(defines.CHAIN_CODE_ETH),
	}
	requireInfo := &MintRequire{
		TokenFrom: 1,
		TokenTo:   2,
		Receiver:  []byte("receiver"),
	}
	info := &MintInfo{
		Event: event,
		Req:   requireInfo,
	}
	recharge := getRecharge(info)
	ethRecharge := &pb.EthRecharge{}
	proto.Unmarshal(recharge, ethRecharge)
	t.Logf("recharge:%v", ethRecharge)
}

func TestProcess(t *testing.T) {
	ctl := gomock.NewController(t)
	srv := business.NewMockIService(ctl)
	srv.EXPECT().SendToSign(gomock.Any())
	srv.EXPECT().SendTx(gomock.Any()).Return(nil)
	srv.EXPECT().IsDone(gomock.Any()).Return(true)
	srv.EXPECT().IsTxOnChain(gomock.Any(), gomock.Any()).Return(true)
	srv.EXPECT().CommitTx(gomock.Any())
	ch := make(chan node.BusinessEvent)
	srv.EXPECT().SubScribe(gomock.Any()).Return(ch)
	tmpDir, _ := ioutil.TempDir("", "p2p")
	defer os.RemoveAll(tmpDir)
	p := NewProcesser(srv, tmpDir)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		ch <- &node.WatchedEvent{}
		wg.Done()
	}()
	p.Run()
	wg.Wait()
}
