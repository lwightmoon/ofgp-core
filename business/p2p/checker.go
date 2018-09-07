package p2p

import (
	"time"

	"github.com/ofgp/ofgp-core/message"
	"github.com/ofgp/ofgp-core/node"
)

type confirmTimeoutChecker struct {
	db               *p2pdb
	interval         time.Duration //执行周期
	confirmTolerance int64         //confirm timeout
	signTimeout      int64         //signTimeout
	service          *service
	signFailTxs      map[string]*WaitConfirmMsg
}

func newConfirmChecker(db *p2pdb, interval time.Duration, confirmTolerance int64) *confirmTimeoutChecker {
	checker := &confirmTimeoutChecker{
		db:               db,
		interval:         interval,
		confirmTolerance: confirmTolerance,
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

func (checker *confirmTimeoutChecker) addSignFailed(wait *WaitConfirmMsg) {
	checker.signFailTxs[wait.ScTxId] = wait
}
func (checker *confirmTimeoutChecker) isInSignFailed(scTxID string) bool {
	_, ok := checker.signFailTxs[scTxID]
	return ok
}
func (checker *confirmTimeoutChecker) delSignFailed(scTxID string) {
	delete(checker.signFailTxs, scTxID)
}
func (checker *confirmTimeoutChecker) retryFailed() {
	for _, waitConfirmTx := range checker.signFailTxs {
		scTxID := waitConfirmTx.ScTxId
		if checker.service.isSignFail(scTxID) { //本term失败
			continue
		} else {
			if checker.db.getSendedInfo(scTxID) != nil || checker.service.isDone(scTxID) {
				checker.delSignFailed(scTxID)
				p2pLogger.Warn("already signed", "scTxID", scTxID)
				continue
			}
			if checker.service.isDone(scTxID) {
				p2pLogger.Warn("already finished", "scTxID", scTxID)
				checker.delSignFailed(scTxID)
				checker.db.clear(scTxID)
			}
			p2pInfo := checker.db.getP2PInfo(scTxID)
			if p2pInfo == nil {
				p2pLogger.Error("check confirm timeout p2pInfo is nil", "scTxID", scTxID)
				checker.delSignFailed(scTxID)
				checker.db.clear(scTxID)
				continue
			}

			newTx, _ := checker.service.createTx(uint8(waitConfirmTx.Opration), p2pInfo)
			if newTx != nil {
				checker.service.sendtoSign(&message.WaitSignMsg{
					Business: p2pInfo.Event.Business,
					ID:       scTxID,
					ScTxID:   scTxID,
					Event:    p2pInfo.Event,
					Tx:       newTx,
				})
			}
			checker.delSignFailed(scTxID)
			waitConfirmTx.Time = time.Now().Unix()
			checker.db.setWaitConfirm(scTxID, waitConfirmTx)
		}
	}
}

func (checker *confirmTimeoutChecker) isSignTimeout(wait *WaitConfirmMsg) bool {
	return time.Now().Unix()-wait.Time > checker.signTimeout
}

func (checker *confirmTimeoutChecker) isConfirmTimeout(waitConfirm *WaitConfirmMsg) bool {
	return (time.Now().Unix() - waitConfirm.Time) > checker.getConfirmTimeout(waitConfirm.Chain)
}

func (checker *confirmTimeoutChecker) check() {
	waitTxs := checker.db.getAllWaitConfirm()
	//retry sign fail
	checker.retryFailed()
	//check sign and confirm 是否超时
	for _, waitConfirm := range waitTxs {
		scTxID := waitConfirm.ScTxId
		//sign timeout
		if !checker.isInSignFailed(scTxID) && checker.isSignTimeout(waitConfirm) && checker.db.getSendedInfo(scTxID) == nil {
			checker.service.markSignFail(scTxID)
			checker.addSignFailed(waitConfirm) //下一个term重试
			checker.service.accuse()
			continue
		}
		//confirm超时 重新发起交易
		if checker.isConfirmTimeout(waitConfirm) && waitConfirm.Info == nil { //confirm超时
			if checker.service.isDone(scTxID) {
				p2pLogger.Warn("already finished", "scTxID", scTxID)
				checker.db.clear(scTxID)
				continue
			}

			p2pInfo := checker.db.getP2PInfo(scTxID)
			if p2pInfo == nil {
				p2pLogger.Error("check confirm timeout p2pInfo is nil", "scTxID", scTxID)
				checker.db.clear(scTxID)
				continue
			}
			sendedInfo := checker.db.getSendedInfo(scTxID)
			checker.node.OnConfirmFail()
			// check交易是否在链上存在 btc bch
			// 存在则返回
			newTx, _ := checker.service.createTx(uint8(waitConfirm.Opration), p2pInfo)
			if newTx != nil {
				checker.service.sendtoSign(&message.WaitSignMsg{
					Business: p2pInfo.Event.Business,
					ID:       scTxID,
					ScTxID:   scTxID,
					Event:    p2pInfo.Event,
					Tx:       newTx,
				})
			}
			waitConfirm.Time = time.Now().Unix()
			checker.db.setWaitConfirm(scTxID, waitConfirm)
			checker.service.accuseWithTerm(sendedInfo.SignTerm)
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
