package p2p

import (
	"time"
)

type matchTimeoutChecker struct {
	checkInterval time.Duration
	db            *p2pdb
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
			//todo创建发送交易
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
