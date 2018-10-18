package node

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/wire"
	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/dgwdb"
	"github.com/ofgp/ofgp-core/primitives"
	pb "github.com/ofgp/ofgp-core/proto"
)

func TestSyncMap(t *testing.T) {
	var m sync.Map
	for i := 0; i < 10; i++ {
		m.Store(i, i)
	}
	m.Range(func(k, v interface{}) bool {
		fmt.Printf("k:%v,v:%v", k, v)
		val := v.(int)
		if val == 7 {
			m.Delete(val)
		}
		return true
	})
	fmt.Println()
	m.Range(func(k, v interface{}) bool {
		fmt.Printf("k:%v,v:%v", k, v)
		return true
	})
}

func TestDoSave(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "p2p")
	if err != nil {
		panic("create tempdir failed")
	}
	defer os.RemoveAll(tmpDir)
	//create p2pdb
	db, _ := dgwdb.NewLDBDatabase(tmpDir, 1, 1)
	bs := primitives.NewBlockStore(db, nil, nil, nil, nil, nil, 0)
	cf := &collectorFactory{
		eth: newEthResCollector(),
	}
	node := &BraftNode{
		blockStore:       bs,
		collectorFactory: cf,
		pubsub:           newPubServer(1),
	}
	business := "p2p"
	go func() {
		ch := node.pubsub.subScribe(business)
		val := <-ch
		t.Logf("result msg:%v", val.GetBusiness())
	}()
	n := 5
	var wg sync.WaitGroup
	wg.Add(n)
	cluster.QuorumN = n
	for i := 0; i < n; i++ {
		txID := "testID"
		signReq := &pb.SignRequest{
			WatchedEvent: &pb.WatchedEvent{
				Business: business,
				To:       2,
				TxID:     txID,
			},
		}
		primitives.SetSignReq(db, signReq, txID)
		signRes := &pb.SignResult{
			Business: "p2p",
			Code:     pb.CodeType_SIGNED,
			NodeID:   int32(i),
			ScTxID:   txID,
			To:       2,
			Term:     0,
			Data:     nil,
		}
		go node.doSave(signRes)
		wg.Done()
	}
	wg.Wait()
	time.Sleep(2 * time.Second)
}

func TestCheckSignTimeout(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "braft_synced")
	if err != nil {
		log.Fatalf("create tmp dir err:%v", err)
	}
	db, _ := dgwdb.NewLDBDatabase(tempDir, cluster.DbCache, cluster.DbFileHandles)

	primitives.InitDB(db, primitives.GenesisBlockPack)
	bs := primitives.NewBlockStore(db, nil, nil, nil, nil, nil, 0)
	ts := primitives.NewTxStore(db)
	go ts.Run(context.TODO())
	node := &BraftNode{
		blockStore: bs,
		txStore:    ts,
	}

	for i := 0; i < 2; i++ {
		idStr := strconv.Itoa(i)
		req := &pb.SignRequest{
			WatchedEvent: &pb.WatchedEvent{
				TxID: idStr,
			},
			Time: 1,
		}
		primitives.SetSignReq(db, req, req.WatchedEvent.TxID)
		node.signedResultCache.Store(idStr, &SignedResultCache{
			initTime: 1,
		})
	}

	node.checkSignTimeout()
	if primitives.GetSignReq(db, "1") != nil {
		t.Error("del fail")
	}
	time.Sleep(time.Second)
	for _, tx := range ts.GetWaitingSignTxs() {
		t.Logf("readd watched id:%s", tx.Msg.ScTxID)
	}

}

func TestCache(t *testing.T) {
	cache := &SignedResultCache{
		cache:       make(map[int32]*pb.SignResult),
		totalCount:  0,
		signedCount: 0,
		initTime:    time.Now().Unix(),
	}
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			cache.addTotalCount()
			if cache.setDone() {
				t.Log("set done")
			}
		}()
	}
	wg.Wait()
	t.Logf("totalcnt:%d", cache.getTotalCount())
}

type tempCollector struct {
}

func (brc *tempCollector) createTx(req *pb.SignRequest) interface{} {
	newlyTx := new(wire.MsgTx)
	return newlyTx
}
func (brc *tempCollector) check(tx interface{}, res *pb.SignResult) bool {
	if newlyTx, ok := tx.(*wire.MsgTx); ok {
		leaderLogger.Info("btc tx type is right", "tx", newlyTx)
		return true
	}
	leaderLogger.Error("btc tx type is wrong")
	return false
}
func TestTempCheck(t *testing.T) {
	temp := &tempCollector{}
	newTx := temp.createTx(nil)
	checkRes := temp.check(newTx, &pb.SignResult{})
	t.Logf("sign check res:%t", checkRes)
}
