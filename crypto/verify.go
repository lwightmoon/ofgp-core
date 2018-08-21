package crypto

import (
	"log"

	"github.com/btcsuite/btcd/btcec"
)

func Verify(pubKey []byte, digest *Digest256, sig []byte) bool {
	pk, err := btcec.ParsePubKey(pubKey, btcec.S256())
	if err != nil {
		return false
	}
	//s := sig[:len(sig)-1]
	pSig, err := btcec.ParseDERSignature(sig, btcec.S256())
	if err != nil {
		log.Printf("veryfy err:%v", err)
		return false
	}
	return pSig.Verify(digest.Data, pk)
}
