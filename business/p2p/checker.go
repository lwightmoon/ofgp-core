package p2p

import (
	"time"

	"github.com/ofgp/ofgp-core/node"
)

type matchTimeoutChecker struct {
	checkInterval  time.Duration
	confirmTimeout int64
	db             *p2pdb
	node           *node.BraftNode
}

func newMatchTimeoutChecker(interval time.Duration, db *p2pdb) *matchTimeoutChecker {
	return &matchTimeoutChecker{
		checkInterval: interval,
		db:            db,
	}
}
func (checker *matchTimeoutChecker) check() {
	infos := checker.db.getAllP2PInfos()
	for _, info := range infos {
		if info.IsExpired() && !isMatched(checker.db, info.GetScTxID()) {
			info.GetBackTxParam()
			//todo创建发送回退交易
			//删除
		}
	}
}

func (checker *matchTimeoutChecker) run() {
	ticker := time.NewTicker(checker.checkInterval).C
	go func() {
		for {
			<-ticker
			checker.check()
		}
	}()
}

type confirmTimeoutChecker struct {
	db       *p2pdb
	interval time.Duration
	timeout  int64
}

func (checker *confirmTimeoutChecker) check() {
	txs := checker.db.getAllSendedTx()
	for _, tx := range txs {
		waitConfirm := checker.db.getWaitConfirm(tx.TxId)
		if waitConfirm == nil { //check未完成时被确认
			p2pLogger.Warn("waitConfirm is nil", "scTxID", tx.TxId)
			continue
		}
		if (time.Now().Unix()-tx.Time) > checker.timeout && waitConfirm.Info == nil { //超时
			p2pInfo := checker.db.getP2PInfo(tx.TxId)
			if p2pInfo == nil {
				p2pLogger.Error("check confirm timeout p2pInfo is nil", "scTxID", tx.TxId)
				continue
			}
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

func (checker *confirmTimeoutChecker) del(scTxID string) {

}
