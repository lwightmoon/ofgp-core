package mint

import (
	proto "github.com/golang/protobuf/proto"
	"github.com/ofgp/ofgp-core/dgwdb"
	"github.com/syndtr/goleveldb/leveldb"
)

var (
	mintInfoPrefix = []byte("mintInfo")
	sended         = []byte("sended")
)

// mintDB 铸币熔币db
type mintDB struct {
	db *dgwdb.LDBDatabase
}

func newMintDB(db *dgwdb.LDBDatabase) *mintDB {
	return &mintDB{
		db: db,
	}
}

func (db *mintDB) setMintInfo(info *MintInfo) {
	txID := info.GetEvent().GetTxID()
	key := append(mintInfoPrefix, []byte(txID)...)
	data, err := proto.Marshal(info)
	if err != nil {
		panic("marshal mint info err")
	}
	db.db.Put(key, data)
}

func (db *mintDB) getMintInfo(scTxID string) *MintInfo {
	key := append(mintInfoPrefix, []byte(scTxID)...)
	data, err := db.db.Get(key)
	if err != nil {
		return nil
	}
	info := &MintInfo{}
	err = proto.Unmarshal(data, info)
	if err != nil {
		panic("unmarshal data err")
	}
	return info
}

func (db *mintDB) existMintInfo(txID string) bool {
	key := append(mintInfoPrefix, []byte(txID)...)
	ldb := db.db.LDB()
	exist, err := ldb.Has(key, nil)
	if err != nil {
		mintLogger.Error("is there mintInfo err", "err", err)
		return exist
	}
	return exist
}

func (db *mintDB) setSended(ScTxID string) {
	key := append(sended, []byte(ScTxID)...)
	db.db.Put(key, []byte{0})
}

func (db *mintDB) isSended(ScTxID string) bool {
	key := append(sended, []byte(ScTxID)...)
	ldb := db.db.LDB()
	exist, err := ldb.Has(key, nil)
	if err != nil {
		mintLogger.Error("is there sended err", "err", err)
		return exist
	}
	return exist
}

func (db *mintDB) clear(scTxID string) {
	batch := new(leveldb.Batch)
	mintInfoKey := append(mintInfoPrefix, []byte(scTxID)...)
	sendedKey := append(sended, []byte(sended)...)
	batch.Delete(mintInfoKey)
	batch.Delete(mintInfoKey)
	ldb := db.db.LDB()
	err := ldb.Write(batch, nil)
	if err != nil {
		mintLogger.Error("clear mint data err", "err", err)
	}
}
