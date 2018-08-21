package proto

import (
	"testing"
	"time"
)

func TestTransaction(t *testing.T) {
	tx1 := &Transaction{
		Business: "p2p",
		Vin: []*PublicTx{
			&PublicTx{
				Chain: 1,
			},
		},
		Vout: []*PublicTx{},
		Time: time.Now().Unix(),
		Data: []byte("test"),
	}
	tx1.UpdateId()
	t.Logf("txID:%v", tx1.TxID)
	tx2 := &Transaction{
		Business: "p2p",
		Vin: []*PublicTx{
			&PublicTx{
				Chain: 1,
			},
		},
		Vout: []*PublicTx{},
		Time: time.Now().Unix(),
		Data: []byte("test"),
	}
	if !tx1.EqualTo(tx2) {
		t.Error("not equal")
	}
}
