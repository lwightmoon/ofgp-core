package node

import (
	"github.com/ofgp/ofgp-core/message"
	pb "github.com/ofgp/ofgp-core/proto"
)

func (node *BraftNode) CreateTx(req CreateReq) *pb.NewlyTx {
	tx := node.txInvoker.CreateTx(req)
	return tx
}

func (node *BraftNode) SignTx(msg *message.WaitSignMsg) {
	node.txStore.AddTxtoWaitSign(msg)
}

func (node *BraftNode) SendTx(req ISendReq) {
	node.txInvoker.SendTx(req)
}

func (node *BraftNode) Commit(req *pb.Transaction) {
	node.txStore.CreateInnerTx(req)
}

// IsDone 判断网关是否处理完成
func (node *BraftNode) IsDone(scTxID string) bool {
	inMem := node.txStore.IsTxInMem(scTxID)
	indb := node.txStore.HasTxInDB(scTxID)
	return inMem || indb
}

func (node *BraftNode) GetTxByHash(txid string) {

}

// CleanSigned 清理签名数据 业务方重试使用
func (node *BraftNode) OnConfirmFail(msg *message.WaitSignMsg, scTxID string, term int64) {
	if node.txStore.IsTxInMem(scTxID) && node.txStore.HasTxInDB(scTxID) {
		return
	}
	//todo
	//调用GetTxByHash()check是否已经发送成功

	node.blockStore.MarkFailedSignRecord(scTxID, term)
	node.signedResultCache.Delete(scTxID)
	node.blockStore.DeleteSignReq(scTxID)
	//删除等待队列
	node.txStore.DeleteWaitSign(scTxID)
	//删除已签名标记
	node.signedTxs.Delete(scTxID)
	node.txStore.AddTxtoWaitSign(msg)

	node.accuser.TriggerByBusiness(term)
}
