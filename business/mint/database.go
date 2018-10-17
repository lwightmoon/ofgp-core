package mint

import "github.com/ofgp/ofgp-core/dgwdb"

var (
	mintInfoPrefix = []byte("mintInfo")
)

type mintDB struct {
	db *dgwdb.LDBDatabase
}

func (db *mintDB) setMintInfo(info *MintInfo) {

}

func (db *mintDB) getMintInfo(scTxID string) *MintInfo {
	return nil
}

func (db *mintDB) existMintInfo(txID string) bool {
	return false
}

func (db *mintDB) setSended(ScTxID string) {

}

func (db *mintDB) isSended(ScTxID string) bool {
	return false
}

func (db *mintDB) clear(scTxID string) {

}
