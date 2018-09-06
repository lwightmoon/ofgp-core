package p2p

import "testing"

func TestList(t *testing.T) {
	list := newRetryList()
	list.addToRetry(&sendingInfo{0, nil})
	infos := list.getSendingInfos()
	t.Logf("size:%d", len(infos))
	for _, info := range infos {
		t.Logf("get info type:%v", info)
	}
	infos = list.getSendingInfos()
	if len(infos) != 0 {
		t.Fail()
	}
	for _, info := range infos {
		t.Logf("get info type:%d", info.TxType)
	}
}
