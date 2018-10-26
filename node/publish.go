package node

import (
	"github.com/ofgp/common/defines"
	pb "github.com/ofgp/ofgp-core/proto"
)

// noticeCommit通知commit
func (bn *BraftNode) pubCommit(blockPack *pb.BlockPack) {
	block := blockPack.Block()
	height := blockPack.Height()
	for index, tx := range block.Txs {
		commitedData := &CommitedData{
			Tx:     tx,
			Height: height,
			Index:  index,
		}
		event := newCommitedEvent(tx.Business, commitedData, nil)
		bn.pubsub.pub(tx.Business, event)
	}
}

//noticeSigned 通知已签名
func (bn *BraftNode) pubSigned(msg *pb.SignResult, chain uint32,
	newTx interface{}, signBeforeTxID string, term int64) {
	signedData := &SignedData{
		Chain: chain,
		ID:    msg.ScTxID,
		TxID:  msg.ScTxID,
		// Term:  term,
		Tx: newTx,
	}
	event := newSignedEvent(msg.Business, signedData, nil)
	bn.pubsub.pub(msg.Business, event)
}

func (bn *BraftNode) pubWatcherEvent(watchedEvent *pb.WatchedEvent) {
	var event BusinessEvent
	switch watchedEvent.GetEventType() {
	case defines.EVENT_COIN_MINT_REQUIRE:
		fallthrough
	case defines.EVENT_P2P_SWAP_REQUIRE: //初始监听到
		event = newBusinessWatchedEvent(watchedEvent)
	case defines.EVENT_COIN_MINT_CONFIRM:
		fallthrough
	case defines.EVENT_P2P_SWAP_CONFIRM: //确认
		event = newConfirmEvent(watchedEvent)
	default:
		nodeLogger.Error("event type err", "type", watchedEvent.GetEventType(), "scTxID", watchedEvent.GetTxID())
		return
	}
	nodeLogger.Info("pub event start", "scTxID", watchedEvent.GetTxID())
	bn.pubsub.pub(event.GetBusiness(), event)
	nodeLogger.Info("pub event end", "scTxID", watchedEvent.GetTxID())
}
