package node

import (
	"testing"

	btcwatcher "github.com/ofgp/bitcoinWatcher/mortgagewatcher"
)

func TestCreate(t *testing.T) {
	bchWatcher, err := btcwatcher.NewMortgageWatcher("bch", 0,
		"", nil, 1000)
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
