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
	waitTxs := checker.db.getAllWaitConfirm()
	for _, waitConfirm := range waitTxs {
		scTxID := waitConfirm.ScTxId
		if (time.Now().Unix()-waitConfirm.Time) > checker.getConfirmTimeout(waitConfirm.Chain) && waitConfirm.Info == nil { //超时

			if checker.node.IsDone(scTxID) {
				p2pLogger.Warn("already finished", "scTxID", scTxID)
				continue
			}
			// waitConfirm := checker.db.getWaitConfirm(scTxID)
			// if waitConfirm == nil {
			// 	p2pLogger.Warn("waitconfirm has been deled", "scTxID", scTxID)
			// 	continue
			// }
			p2pInfo := checker.db.getP2PInfo(scTxID)
			if p2pInfo == nil {
				p2pLogger.Error("check confirm timeout p2pInfo is nil", "scTxID", scTxID)
				continue
			}
			// sendedInfo := checker.db.getSendedInfo(scTxID)
			// checker.node.OnConfirmFail()
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

func (checker *confirmTimeoutChecker) run() {
	ticker := time.NewTicker(checker.interval).C
	go func() {
		for {
			<-ticker
			checker.check()
		}
	}()
}
