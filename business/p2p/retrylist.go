package p2p

import (
	"sync"
)

type sendingInfo struct {
	TxType int //back or match
	info   *P2PInfo
}

type retryList struct {
	sync.Mutex
	infos []*sendingInfo
}

func newRetryList() *retryList {
	return &retryList{
		infos: make([]*sendingInfo, 0),
	}
}

func (list *retryList) getSendingInfos() []*sendingInfo {
	list.Lock()
	defer list.Unlock()
	newinfos := make([]*sendingInfo, len(list.infos))
	copy(newinfos, list.infos)
	list.infos = make([]*sendingInfo, 0)
	return newinfos
}

func (list *retryList) addToRetry(info *sendingInfo) {
	list.Lock()
	defer list.Unlock()
	list.infos = append(list.infos, info)
}
