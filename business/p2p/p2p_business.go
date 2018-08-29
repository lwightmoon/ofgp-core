package p2p

import (
	"fmt"
	"sync"
	"time"

	"github.com/ofgp/ofgp-core/log"

	"github.com/ofgp/ofgp-core/business"
	"github.com/ofgp/ofgp-core/node"
	pb "github.com/ofgp/ofgp-core/proto"
	"github.com/spf13/viper"
)

var p2pLogger = log.New(viper.GetString("loglevel"), "node")

type P2P struct {
	ch      chan node.BusinessEvent
	node    *node.BraftNode
	handler business.IHandler
}

const businessName = "p2p"

func NewP2P(node *node.BraftNode, db *p2pdb) *P2P {
	p2p := &P2P{
		node: node,
	}
	//向node订阅业务相关事件
	ch := node.SubScribe(businessName)
	p2p.ch = ch

	//初始化处理链
	wh := &watchedHandler{
		db:   db,
		node: node,
	}
	sh := &sigenedHandler{}
	confirmH := &confirmHandler{
		db:   db,
		node: node,
	}
	commitH := &commitHandler{}
	wh.SetSuccessor(sh)
	sh.SetSuccessor(confirmH)
	confirmH.SetSuccessor(commitH)
	p2p.handler = wh
	return p2p
}

func (p2p *P2P) processEvent() {
	for event := range p2p.ch {
		p2p.handler.HandleEvent(event)
	}
}

type watchedHandler struct {
	sync.Mutex
	db    *p2pdb
	index *txIndex
	node  *node.BraftNode
	business.Handler
}

// // signTx 签名交易
// func (wh *watchedHandler) sendToSignTx(tx *P2PTx, seqID []byte) {
// 	wh.db.setP2PTx(tx, seqID)
// 	//todo 创建交易，并交给网关签名
// 	p2pLogger.Debug("create tx and send to sign")
// }

// getP2PInfo p2p交易数据
func getP2PInfo(event *pb.WatchedEvent) *P2PInfo {
	msg := &p2pMsg{}
	msg.Decode(event.GetData())
	p2pMsg := msg.toPBMsg()
	info := &P2PInfo{
		Event: event,
		Msg:   p2pMsg,
	}
	return info
}

func (wh *watchedHandler) HandleEvent(event node.BusinessEvent) {
	if watchedEvent, ok := event.(*node.WatchedEvent); ok {
		txEvent := watchedEvent.GetData()
		if event == nil {
			p2pLogger.Error("data is nil", "business", watchedEvent.GetBusiness())
			return
		}
		info := getP2PInfo(txEvent)
		wh.db.setP2PInfo(info)
		wh.index.Add(info)
		chain, addr, amount := info.getExchangeInfo()
		// 使用要求的数据匹配交易数据
		txIDs := wh.index.GetTxID(chain, addr, amount)
		if txIDs == nil || len(txIDs) == 0 {
			for _, txID := range txIDs {
				matchedInfo := wh.db.getP2PInfo(txID, chain)
				if matchedInfo == nil {
					p2pLogger.Error("get p2pInfo nil", "business", event.GetBusiness(), "scTxID", txEvent.GetTxID())
					continue
				}
				p2pLogger.Debug("handle watchedEvent", "create tx and send to sign")
				break
			}
		}

	} else if wh.Successor != nil {
		wh.Successor.HandleEvent(event)
	}

}

type sigenedHandler struct {
	business.Handler
}

func (sh *sigenedHandler) HandleEvent(event node.BusinessEvent) {
	if signedEvent, ok := event.(*node.SignedEvent); ok {
		p2pLogger.Info("handle signed")
		signedData := signedEvent.GetData()
		if signedData == nil {
			p2pLogger.Error("signed data is nil")
			return
		}
		p2pLogger.Debug("receive signedData", "scTxID", signedData.ID)
		//todo 发送交易
		p2pLogger.Debug("------sendTx")

	} else if sh.Successor != nil {
		sh.Successor.HandleEvent(event)
	}
}

type confirmHandler struct {
	sync.Mutex
	db *p2pdb
	business.Handler
	node *node.BraftNode
}

// getPubTxFromInfo huoqu g
func getPubTxFromInfo(info *P2PInfo) *pb.PublicTx {
	if info == nil {
		return nil
	}
	event := info.GetEvent()
	pubTx := &pb.PublicTx{
		Chain:  event.GetFrom(),
		TxID:   event.GetTxID(),
		Amount: int64(event.GetAmount()),
		Data:   event.GetData(),
	}
	return pubTx
}
func getVin(infos []*P2PInfo) []*pb.PublicTx {
	pubtxs := make([]*pb.PublicTx, 0)
	for _, info := range infos {
		pubtx := getPubTxFromInfo(info)
		if pubtx != nil {
			pubtxs = append(pubtxs, pubtx)
		}
	}
	return pubtxs
}

func getPubTxFromConfirmInfo(info *P2PConfirmInfo) *pb.PublicTx {
	if info == nil {
		return nil
	}
	event := info.GetEvent()
	pubTx := &pb.PublicTx{
		Chain:  event.GetFrom(),
		TxID:   event.GetTxID(),
		Amount: int64(event.GetAmount()),
		Data:   event.GetData(),
	}
	return pubTx
}

func getVout(infos []*P2PConfirmInfo) []*pb.PublicTx {

	pubtxs := make([]*pb.PublicTx, 0)
	for _, info := range infos {
		pubtx := getPubTxFromConfirmInfo(info)
		if pubtx != nil {
			pubtxs = append(pubtxs, pubtx)
		}
	}
	return pubtxs
}

// createDGWTx 创建网关交易
func createDGWTx(business string, infos []*P2PInfo, confirmInfos []*P2PConfirmInfo) *pb.Transaction {
	tx := &pb.Transaction{
		Business: business,
		Vin:      getVin(infos),
		Vout:     getVout(confirmInfos),
		Time:     time.Now().Unix(),
	}
	tx.UpdateId()
	return tx
}

// commitTx commit点对点交易
func (handler *confirmHandler) commitTx(business string, infos []*P2PInfo, confirmInfos []*P2PConfirmInfo) {
	p2pLogger.Debug("commit dgw tx", "business", business)
	dgwTx := createDGWTx(business, infos, confirmInfos)
	handler.node.Commit(dgwTx)
}

func getP2PConfirmInfo(event *pb.WatchedEvent) *P2PConfirmInfo {
	msg := &p2pMsgConfirmed{}
	msg.Decode(event.GetData())
	pbMsg := msg.toPBMsg()
	info := &P2PConfirmInfo{
		Event: event,
		Msg:   pbMsg,
	}
	return info
}
func (handler *confirmHandler) HandleEvent(event node.BusinessEvent) {
	if confirmedEvent, ok := event.(*node.ConfirmEvent); ok {
		txEvent := confirmedEvent.GetData()
		if event == nil {
			p2pLogger.Error("confirm data is nil")
			return
		}
		//交易确认info
		info := getP2PConfirmInfo(txEvent)
		p2pLogger.Info("handle confirm", "scTxID", info.Msg.Id)
		oldTxID := info.Msg.Id
		if info.Msg.Opration == confirmed { //确认交易 需要等待发起和匹配交易确认
			waitConfirm := handler.db.getWaitConfirm(oldTxID)
			waitConfirm.Info = info
			handler.db.setWaitConfirm(oldTxID, waitConfirm)

			//先前匹配的交易id
			oldMatchedTxID := waitConfirm.GetMatchedTxId()

			oldWaitConfirm := handler.db.getWaitConfirm(oldMatchedTxID)
			if oldWaitConfirm.Info != nil { //exchange两笔交易都已confirm
				confirmInfos := []*P2PConfirmInfo{info, oldWaitConfirm.Info}
				oldTxIDs := []string{oldTxID, oldMatchedTxID}
				p2pInfos := handler.db.getP2PInfos(oldTxIDs)
				handler.commitTx(event.GetBusiness(), p2pInfos, confirmInfos)
			} else {
				p2pLogger.Info("wait another tx confirm", "scTxID", oldTxID)
			}

		} else { //回退交易 commit当前confirmInfo和对应的p2pInfo
			confirmInfos := []*P2PConfirmInfo{info}
			oldTxIDs := []string{oldTxID}
			p2pInfos := handler.db.getP2PInfos(oldTxIDs)
			handler.commitTx(event.GetBusiness(), p2pInfos, confirmInfos)
		}

	} else if handler.Successor != nil {
		handler.Successor.HandleEvent(event)
	}
}

type commitHandler struct {
	business.Handler
}

func (ch *commitHandler) HandleEvent(event node.BusinessEvent) {
	if val, ok := event.(*node.CommitedEvent); ok {
		commitedData := val.GetData()
		if commitedData == nil {
			p2pLogger.Error("commit data is nil")
			return
		}
		fmt.Printf("commitdata:%v", commitedData)
		p2pLogger.Info("handle Commited", "innerTxID", commitedData.TxID.ToText())
	} else {
		p2pLogger.Error("could not handle event")
	}
}
