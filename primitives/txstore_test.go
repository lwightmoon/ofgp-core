package primitives

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/dgwdb"
	pb "github.com/ofgp/ofgp-core/proto"
)

func TestInnerTxStore(t *testing.T) {
	its := newInnerTxStore()
	vin1 := []*pb.PublicTx{
		&pb.PublicTx{
			TxID: "vin1",
		},
	}
	vout1 := []*pb.PublicTx{
		&pb.PublicTx{
			TxID: "vout1",
		},
	}
	vin2 := []*pb.PublicTx{
		&pb.PublicTx{
			TxID: "vin2",
		},
	}
	vout2 := []*pb.PublicTx{
		&pb.PublicTx{
			TxID: "vout2",
		},
	}
	txID1 := &crypto.Digest256{
		Data: []byte("0"),
	}
	txID2 := &crypto.Digest256{
		Data: []byte("1"),
	}
	tx1 := &pb.Transaction{
		TxID:     txID1,
		Business: "p2p",
		Vin:      vin1,
		Vout:     vout1,
		Time:     time.Now().Unix(),
	}
	tx2 := &pb.Transaction{
		TxID:     txID2,
		Business: "p2p",
		Vin:      vin2,
		Vout:     vout2,
		Time:     time.Now().Unix(),
	}
	txtime1 := &txWithTimeMs{
		tx: tx1,
	}
	txtime2 := &txWithTimeMs{
		tx: tx2,
	}
	its.addTx(txtime1)
	its.addTx(txtime2)
	queryID := &crypto.Digest256{
		Data: []byte("0"),
	}
	res := its.getTxByTxID(queryID)
	if res == nil {
		t.Error("not exit txid")
	}
	tx := its.getByPubTxID("vin1")
	if tx == nil {
		t.Error("get byPubTxId nil")
	}
	t.Logf("id:%s", tx.tx.TxID.ToText())
	txs := its.getTxs()
	if len(txs) == 0 {
		t.Error("getTxs fail")
	}
	for _, tx := range txs {
		t.Logf("txID:%s", tx.TxID.ToText())
	}
	tx2Del := txs[0]
	its.delTx(tx2Del)

	txQuery := its.getTxByTxID(tx2Del.TxID)
	if txQuery != nil {
		t.Error("del fail")
	}
	txQuery = its.getByPubTxID(tx2Del.Vin[0].TxID)
	if txQuery != nil {
		t.Error("del index fail")
	}

}

var txStore *TxStore

func TestMain(m *testing.M) {
	tmpDir, _ := ioutil.TempDir("", "testTxStore")
	fmt.Printf("tmpdir:%s\n", tmpDir)
	defer os.RemoveAll(tmpDir)
	db, _ := dgwdb.NewLDBDatabase(tmpDir, 1, 1)
	txStore = NewTxStore(db)
	ctx, cancel := context.WithCancel(context.Background())
	go txStore.Run(ctx)
	defer cancel()
	exit := m.Run()
	os.Exit(exit)
}

func TestWatchedEvent(t *testing.T) {
	event := &pb.WatchedEvent{
		TxID: "test",
	}
	txStore.AddWatchedEvent(event)
	time.Sleep(1 * time.Millisecond)
	has := txStore.HasWatchedEvent(event)
	if !has{
		t.Errorf("has watched:%v", has)
	}
}
