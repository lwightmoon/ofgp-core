package p2p

import (
	"log"

	"github.com/golang/protobuf/proto"
	"github.com/ofgp/ofgp-core/dgwdb"
)

var (
	p2pInfoPrefix = []byte("p2pInfo")
)

type p2pdb struct {
	db *dgwdb.LDBDatabase
}

// getID 获取db存储key

func (db *p2pdb) setP2PInfo(tx *P2PInfo) {
	txID := tx.Event.TxID
	key := append(p2pInfoPrefix, []byte(txID)...)
	data, err := proto.Marshal(tx)
	if err != nil {
		log.Printf("set P2PInfo err:%v", err)
		return
	}
	db.db.Put(key, data)
}

func (db *p2pdb) getP2PInfo(txID string) *P2PInfo {
	key := append(p2pInfoPrefix, []byte(txID)...)
	data, err := db.db.Get(key)
	if err != nil {
		log.Printf("get P2PInfo err:%v", err)
		return nil
	}
	info := &P2PInfo{}
	err = proto.Unmarshal(data, info)
	if err != nil {
		log.Printf("decode P2PInfo from db err:%v", err)
		return nil
	}
	return info
}
func (db *p2pdb) getP2PInfos(txIDs []string) []*P2PInfo {
	infos := make([]*P2PInfo, 0)
	for _, txID := range txIDs {
		info := db.getP2PInfo(txID)
		if info != nil {
			infos = append(infos, info)
		}
	}
	return infos
}
func (db *p2pdb) delP2PInfo(txID string) {
	key := append(p2pInfoPrefix, []byte(txID)...)
	err := db.db.Delete(key)
	if err != nil {
		p2pLogger.Error("p2p db err", "err", err, "scTxID", txID)
	}
}

// 保存等待确认的交易
func (db *p2pdb) setWaitConfirm(txID string, msg *WaitConfirmMsg) {
	key := append(p2pInfoPrefix, []byte(txID)...)
	data, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("set P2PInfo err:%v", err)
		return
	}
	db.db.Put(key, data)
}

func (db *p2pdb) getWaitConfirm(txID string) *WaitConfirmMsg {
	key := append(p2pInfoPrefix, []byte(txID)...)
	data, err := db.db.Get(key)
	if err != nil {
		log.Printf("get waitConfirm err:%v", err)
		return nil
	}
	msg := &WaitConfirmMsg{}
	err = proto.Unmarshal(data, msg)
	if err != nil {
		log.Printf("decode P2PInfo from db err:%v", err)
		return nil
	}
	return msg
}
