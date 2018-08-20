package node

import (
	"github.com/btcsuite/btcd/wire"
	pb "github.com/ofgp/ofgp-core/proto"
)

// noticeCommit通知commit
func (bn *BraftNode) noticeCommit(blockPack *pb.BlockPack) {
	block := blockPack.Block()
	height := blockPack.Height()
	for index, tx := range block.Txs {
		commitedData := &CommitedData{
			TxID:   tx.TxID,
			Height: height,
			Index:  index,
		}
		event := newCommitedEvent(tx.Business, commitedData, nil)
		bn.pubsub.pub(tx.Business, event)
	}
}

//noticeSigned 通知已签名
func (bn *BraftNode) noticeSigned(msg *pb.SignResult, chain uint32, newTx *wire.MsgTx) {
	signedData := SignedData{
		Chain: chain,
		ID:    msg.ScTxID,
		TxID:  msg.ScTxID,
		Data:  newTx,
	}
	event := newSignedEvent(msg.Business, signedData, nil)
	bn.pubsub.pub(msg.Business, event)
}

func (bn *BraftNode) noticeConfirmed() {

}
