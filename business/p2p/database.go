package p2p

import (
	"log"

	"github.com/golang/protobuf/proto"
	"github.com/ofgp/ofgp-core/dgwdb"
	"github.com/syndtr/goleveldb/leveldb"
)

var (
	p2pInfoPrefix     = []byte("p2pInfo")
	waitConfirmPrefix = []byte("waitConfirm")
	matchedPrefix     = []byte("matched")
	sendedPrefix      = []byte("sended")
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
		p2pLogger.Error("set P2PInfo", "err", err)
		return
	}
	db.db.Put(key, data)
}

func (db *p2pdb) getP2PInfo(txID string) *P2PInfo {
	key := append(p2pInfoPrefix, []byte(txID)...)
	data, err := db.db.Get(key)
	if err != nil {
		p2pLogger.Error("get P2PInfo", "err", err)
		return nil
	}
	info := &P2PInfo{}
	err = proto.Unmarshal(data, info)
	if err != nil {
		p2pLogger.Error("decode P2PInfo from db ", "err", err)
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
		p2pLogger.Error("del p2pInfo", "err", err, "scTxID", txID)
	}
}

// getAllP2PInfos 获取所有p2p交易数据
func (db *p2pdb) getAllP2PInfos() []*P2PInfo {
	infos := make([]*P2PInfo, 0)
	iter := db.db.NewIteratorWithPrefix(p2pInfoPrefix)
	var err error
	for iter.Next() {
		info := &P2PInfo{}
		err = proto.Unmarshal(iter.Value(), info)
		if err != nil {
			p2pLogger.Error("get p2pInfos unmarshal err", "err", err)
			continue
		}
		infos = append(infos, info)
	}
	defer iter.Release()
	return infos
}

// 保存等待确认的交易
func (db *p2pdb) setWaitConfirm(txID string, msg *WaitConfirmMsg) {
	key := append(waitConfirmPrefix, []byte(txID)...)
	data, err := proto.Marshal(msg)
	if err != nil {
		p2pLogger.Error("set P2PInfo err", "err", err)
		return
	}
	db.db.Put(key, data)
}

func (db *p2pdb) getWaitConfirm(txID string) *WaitConfirmMsg {
	key := append(waitConfirmPrefix, []byte(txID)...)
	data, err := db.db.Get(key)
	if err != nil {
		// p2pLogger.Error("get waitConfirm", "err", err, "scTxID", txID)
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

// getAllWaitConfirm 获取所有waitconfirm
func (db *p2pdb) getAllWaitConfirm() []*WaitConfirmMsg {
	waits := make([]*WaitConfirmMsg, 0)
	iter := db.db.NewIteratorWithPrefix(waitConfirmPrefix)
	for iter.Next() {
		wait := &WaitConfirmMsg{}
		proto.Unmarshal(iter.Value(), wait)
		waits = append(waits, wait)
	}
	return waits
}

func (db *p2pdb) delWaitConfirm(txID string) {
	key := append(waitConfirmPrefix, []byte(txID)...)
	err := db.db.Delete(key)
	if err != nil {
		p2pLogger.Error("del waitConfirm", "err", err, "scTxID", txID)
	}
}

// 设置匹配的txID
func (db *p2pdb) setMatched(txID1, txID2 string) {
	batch := new(leveldb.Batch)
	key1 := append(matchedPrefix, []byte(txID1)...)
	key2 := append(matchedPrefix, []byte(txID2)...)
	batch.Put(key1, []byte(txID2))
	batch.Put(key2, []byte(txID1))
	err := db.db.LDB().Write(batch, nil)
	if err != nil {
		p2pLogger.Error("clear data err", "err", err)
	}
}
func (db *p2pdb) setMatchedOne(txID1, txID2 string) {
	key := append(matchedPrefix, []byte(txID1)...)
	db.db.Put(key, []byte(txID2))
}

func (db *p2pdb) getMatched(txID string) string {
	key := append(matchedPrefix, []byte(txID)...)
	matched, _ := db.db.Get(key)
	return string(matched)
}
func (db *p2pdb) ExistMatched(txID string) bool {
	key := append(matchedPrefix, []byte(txID)...)
	mydb := db.db.LDB()
	exist, _ := mydb.Has(key, nil)
	return exist
}

func (db *p2pdb) delMatched(txID string) {
	key := append(matchedPrefix, []byte(txID)...)
	err := db.db.Delete(key)
	if err != nil {
		p2pLogger.Error("delMatched err", "err", err, "scTxID", txID)
	}
}

// 设置等待check
func (db *p2pdb) setSendedInfo(tx *SendedInfo) {
	key := append(sendedPrefix, []byte(tx.TxId)...)
	data, err := proto.Marshal(tx)
	if err != nil {
		p2pLogger.Error("marshal sended err", "err", err, "scTxID", tx.TxId)
		return
	}
	err = db.db.Put(key, data)
	if err != nil {
		p2pLogger.Error("set sended err", "err", err, "scTxID", tx.TxId)
	}
}

func (db *p2pdb) getSendedInfo(txID string) *SendedInfo {
	key := append(sendedPrefix, []byte(txID)...)
	data, err := db.db.Get(key)
	if err != nil {
		return nil
	}
	sendedInfo := &SendedInfo{}
	proto.Unmarshal(data, sendedInfo)
	return sendedInfo
}
func (db *p2pdb) existSendedInfo(txID string) bool {
	key := append(sendedPrefix, []byte(txID)...)
	mydb := db.db.LDB()
	exist, _ := mydb.Has(key, nil)
	return exist
}

func (db *p2pdb) clear(scTxID string) {
	batch := new(leveldb.Batch)
	idbytes := []byte(scTxID)
	p2pInfo := append(p2pInfoPrefix, idbytes...)
	waitConfirm := append(waitConfirmPrefix, idbytes...)
	matched := append(matchedPrefix, idbytes...)
	sended := append(sendedPrefix, idbytes...)
	batch.Delete(p2pInfo)
	batch.Delete(waitConfirm)
	batch.Delete(matched)
	batch.Delete(sended)
	err := db.db.LDB().Write(batch, nil)
	if err != nil {
		p2pLogger.Error("clear data err", "err", err)
	}
}
