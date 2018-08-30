package p2p

import (
	"encoding/json"
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
	ch           chan node.BusinessEvent
	node         *node.BraftNode
	handler      business.IHandler
	matchChecker *matchTimeoutChecker
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
		db:    db,
		node:  node,
		index: newTxIndex(),
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
	p2p.matchChecker = newMatchTimeoutChecker(10*time.Second, db)
	p2p.matchChecker.run()
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
func (wh *watchedHandler) sendToSignTx(infos []*P2PInfo) {
	//todo 创建交易，并交给网关签名
	for _, info := range infos {
		p2pLogger.Debug("create tx and send to sign", "scTxID", info.GetScTxID())
	}
}

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

// setWaitConfirm 设置 txID 和 matchedTxID 的双向对应关系
func (wh *watchedHandler) setWaitConfirm(txID, matchedTxID string) {
	waitConfirmMsg := &WaitConfirmMsg{
		MatchedTxId: matchedTxID,
	}
	wh.db.setWaitConfirm(txID, waitConfirmMsg)
	waitConfirmMsg.MatchedTxId = txID
	wh.db.setWaitConfirm(matchedTxID, waitConfirmMsg)
}

// 判断交易是否已被匹配
func isMatched(db *p2pdb, txID string) bool {
	return db.getWaitConfirm(txID) != nil
}
func (wh *watchedHandler) HandleEvent(event node.BusinessEvent) {
	if watchedEvent, ok := event.(*node.WatchedEvent); ok {
		txEvent := watchedEvent.GetData()
		if event == nil {
			p2pLogger.Error("data is nil", "business", watchedEvent.GetBusiness())
			return
		}
		if isMatched(wh.db, txEvent.GetTxID()) {
			p2pLogger.Warn("tx has already been matched", "scTxID", txEvent.GetTxID)
			return
		}
		info := getP2PInfo(txEvent)
		if info.IsExpired() { //匹配交易超时
			p2pLogger.Warn("tx has already been expired", "scTxID", txEvent.GetTxID)
			//todo 发送回退交易
			return
		}
		wh.db.setP2PInfo(info)
		p2pLogger.Debug("add coniditon", "chian", info.Event.From, "addr", info.Msg.SendAddr, "amount", info.Event.Amount)
		wh.index.Add(info)
		//要求匹配的条件
		chain, addr, amount := info.getExchangeInfo()
		p2pLogger.Debug("search coniditon", "chian", chain, "addr", addr, "amount", amount)
		// 使用要求的数据匹配交易数据
		txIDs := wh.index.GetTxID(chain, addr, amount)
		p2pLogger.Debug("matchIDs", "txIDs", txIDs)
		if txIDs != nil {
			for _, txID := range txIDs {
				matchedInfo := wh.db.getP2PInfo(txID)
				if matchedInfo == nil {
					p2pLogger.Error("get p2pInfo nil", "business", event.GetBusiness(), "scTxID", txEvent.GetTxID())
					continue
				}
				if matchedInfo.IsExpired() { //被匹配记录超时
					p2pLogger.Warn("tx matched has already been matched", "scTxID", txEvent.GetTxID)
					//todo 发送回退交易
					continue
				}
				infos := []*P2PInfo{info, matchedInfo}
				//创建交易发送等地啊签名
				wh.sendToSignTx(infos)

				//保存已匹配的两笔交易的txid
				wh.setWaitConfirm(info.GetScTxID(), matchedInfo.GetScTxID())
				//删除索引 防止重复匹配
				wh.index.Del(info)
				break
			}
		} else {
			p2pLogger.Debug("handle watchedEvent not matched", "scTxID", txEvent.GetTxID())
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
	txJSON, _ := json.Marshal(dgwTx)
	p2pLogger.Debug("commit data", "data", string(txJSON))
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

func (handler *confirmHandler) cleanOnConfirmed(infos []*P2PInfo) {
	for _, info := range infos {
		handler.db.delP2PInfo(info.GetScTxID())
		handler.db.delWaitConfirm(info.GetScTxID())
	}
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
		//之前的交易id
		oldTxID := info.Msg.Id
		if info.Msg.Opration == confirmed { //确认交易 需要等待发起和匹配交易确认
			waitConfirm := handler.db.getWaitConfirm(oldTxID)
			if waitConfirm == nil {
				p2pLogger.Error("never matched tx", "scTxID", oldTxID)
				return
			}
			waitConfirm.Info = info
			handler.db.setWaitConfirm(oldTxID, waitConfirm)

			//先前匹配的交易id
			oldMatchedTxID := waitConfirm.GetMatchedTxId()
			oldWaitConfirm := handler.db.getWaitConfirm(oldMatchedTxID)
			if oldWaitConfirm.Info != nil { //与oldTxID匹配的交易已被confirm
				confirmInfos := []*P2PConfirmInfo{info, oldWaitConfirm.Info}
				oldTxIDs := []string{oldTxID, oldMatchedTxID}
				p2pLogger.Debug("get old info", "txIDs", oldTxIDs)
				p2pInfos := handler.db.getP2PInfos(oldTxIDs)
				handler.commitTx(event.GetBusiness(), p2pInfos, confirmInfos)
				handler.cleanOnConfirmed(p2pInfos)
			} else {
				p2pLogger.Info("wait another tx confirm", "scTxID", oldTxID)
			}

		} else if info.Msg.Opration == back { //回退交易 commit当前confirmInfo和对应的p2pInfo
			p2pLogger.Debug("hanle confirm back", "scTxID", info.Msg.Id)
			confirmInfos := []*P2PConfirmInfo{info}
			oldTxIDs := []string{oldTxID}
			p2pInfos := handler.db.getP2PInfos(oldTxIDs)
			handler.commitTx(event.GetBusiness(), p2pInfos, confirmInfos)
			handler.cleanOnConfirmed(p2pInfos)
		} else {
			p2pLogger.Error("oprationtype wrong", "opration", info.Msg.Opration)
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
