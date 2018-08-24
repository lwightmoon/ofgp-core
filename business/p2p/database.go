package p2p

import (
	"log"

	"github.com/golang/protobuf/proto"
	"github.com/ofgp/ofgp-core/dgwdb"
)

var (
	p2pTxPrefix   = []byte("p2pTx")
	txIDMapPrefix = []byte("txIDMap")
	confirmPrefix = []byte("confirmed")
)

type p2pdb struct {
	db *dgwdb.LDBDatabase
}

func (db *p2pdb) setP2PTx(tx *P2PTx, seqID []byte) {
	key := append(p2pTxPrefix, seqID...)
	data, err := proto.Marshal(tx)
	if err != nil {
		log.Printf("set p2ptx err:%v", err)
		return
	}
	db.db.Put(key, data)
}

func (db *p2pdb) getP2PTx(seqID []byte) *P2PTx {
	key := append(p2pTxPrefix, seqID...)
	data, err := db.db.Get(key)
	if err != nil {
		log.Printf("get p2ptx err:%v", err)
		return nil
	}
	tx := &P2PTx{}
	proto.Unmarshal(data, tx)
	return tx
}

func (db *p2pdb) setP2PNewTx(tx *P2PNewTx, seqID []byte) {
	key := append(confirmPrefix, seqID...)
	data, err := proto.Marshal(tx)
	if err != nil {
		log.Printf("set p2pNewtx err:%v", err)
		return
	}
	db.db.Put(key, data)
}

func (db *p2pdb) getP2pNewTx(seqID []byte) *P2PNewTx {
	key := append(confirmPrefix, seqID...)
	data, err := db.db.Get(key)
	if err != nil {
		log.Printf("get p2ptx err:%v", err)
		return nil
	}
	tx := &P2PNewTx{}
	proto.Unmarshal(data, tx)
	return tx
}

func (db *p2pdb) setTxSeqIDMap(txID string, seqID []byte) {
	key := append(txIDMapPrefix, []byte(txID)...)
	db.db.Put(key, seqID)
}

func (db *p2pdb) getTxSeqID(txID string) []byte {
	key := append(txIDMapPrefix, []byte(txID)...)
	data, err := db.db.Get(key)
	if err != nil {
		log.Printf("get seqID err:%v", err)
	}
	return data
}
