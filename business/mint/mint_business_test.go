package mint

import (
	"testing"

	proto "github.com/golang/protobuf/proto"
	"github.com/ofgp/common/defines"
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
