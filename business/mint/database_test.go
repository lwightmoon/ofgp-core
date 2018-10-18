package mint

import (
	"testing"

	pb "github.com/ofgp/ofgp-core/proto"
)

func TestDB(t *testing.T) {
	info := &MintInfo{
		Event: &pb.WatchedEvent{
			TxID: "testTxID",
		},
		Req: &MintRequire{
			TokenFrom: 1,
			TokenTo:   2,
			Receiver:  []byte("testreceiver"),
		},
	}
	mintdb.setMintInfo(info)
	newinfo := mintdb.getMintInfo("testTxID")
	if newinfo == nil {
		t.Error("get nil")
	}
	exist := mintdb.existMintInfo("testTxID")
	if !exist {
		t.Error("exist err")
	}
}
