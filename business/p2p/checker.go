package p2p

import (
	"time"

	"github.com/ofgp/ofgp-core/message"
	"github.com/ofgp/ofgp-core/node"
)

type confirmTimeoutChecker struct {
	db               *p2pdb
	interval         time.Duration
	confirmTolerance int64
	node             *node.BraftNode
}

func newConfirmChecker(db *p2pdb, interval time.Duration, confirmTolerance int64, node *node.BraftNode) *confirmTimeoutChecker {
	checker := &confirmTimeoutChecker{
		db:               db,
		interval:         interval,
		confirmTolerance: confirmTolerance,
		node:             node,
	}
	checker.run()
	return checker
}

func (checker *confirmTimeoutChecker) getConfirmTimeout(chain uint32) int64 {
	switch chain {
	case message.Bch:
		fallthrough
	case message.Btc:
		return int64(node.BchConfirms)*60 + checker.confirmTolerance
	case message.Eth:
		return int64(node.BchConfirms)*15 + checker.confirmTolerance
	default:
		return checker.confirmTolerance
	}

}

func (checker *confirmTimeoutChecker) check() {
	txs := checker.db.getAllSendedTx()
	for _, tx := range txs {
		waitConfirm := checker.db.getWaitConfirm(tx.TxId)
		if waitConfirm == nil { //check未完成时被确认
			p2pLogger.Warn("waitConfirm is nil", "scTxID", tx.TxId)
			continue
		}
		if (time.Now().Unix()-tx.Time) > checker.getConfirmTimeout(waitConfirm.Chain) && waitConfirm.Info == nil { //超时
			p2pInfo := checker.db.getP2PInfo(tx.TxId)
			if p2pInfo == nil {
				p2pLogger.Error("check confirm timeout p2pInfo is nil", "scTxID", tx.TxId)
				continue
			}
			//check交易是否在链上存在
			//存在则返回
			switch waitConfirm.Opration {
			case confirmed: //匹配交易
				//todo
				//重新发起匹配交易

			case back: //回退交易
				//todo
				//重新发起回退交易
			}
			//todo 发起accuse 交易确认超时
		}
	}
}

// 将已经发送的交易加入check队列
func (checker *confirmTimeoutChecker) add(tx *SendedTx) {
	checker.db.setSendedTx(tx)
}

func (checker *confirmTimeoutChecker) run() {
	ticker := time.NewTicker(checker.interval).C
	go func() {
		for {
			<-ticker
			checker.check()
		}
	}()
}
