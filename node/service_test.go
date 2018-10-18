package node

import (
	btwatcher "swap/btwatcher"
	"testing"

	"github.com/ofgp/common/defines"
)

func TestCreate(t *testing.T) {
	bchWatcher, err := btwatcher.NewWatcher(defines.CHAIN_CODE_BCH)
	if err != nil {
		t.Fatalf("create watcher err:%v", err)
	}
	bchOP := newBchOprator(bchWatcher)
	invoker := newTxInvoker(nil, bchOP, nil)
	node := &BraftNode{
		txInvoker: invoker,
	}
	node.CreateTx(nil)
}
