package p2p

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ofgp/ofgp-core/log"
	"github.com/ofgp/ofgp-core/message"

	"github.com/ofgp/ofgp-core/business"
	"github.com/ofgp/ofgp-core/node"
	pb "github.com/ofgp/ofgp-core/proto"
)

var p2pLogger = log.New("DEBUG", "node")

type P2P struct {
	ch             chan node.BusinessEvent
	node           *node.BraftNode
	handler        business.IHandler
	confirmChecker *confirmTimeoutChecker
}

const businessName = "p2p"

func NewP2P(node *node.BraftNode, db *p2pdb) *P2P {
	p2p := &P2P{
		node: node,
	}
	//向node订阅业务相关事件
	ch := node.SubScribe(businessName)
	p2p.ch = ch

	//创建交易匹配索引
	p2pInfos := db.getAllP2PInfos()
	index := newTxIndex()

	index.AddInfos(p2pInfos)
	service := newService(node)
	wh := &watchedHandler{
		db:                 db,
		node:               node,
		index:              index,
		checkMatchInterval: time.Duration(1) * time.Second,
		service:            service,
	}
	// check匹配超时
	wh.runCheckMatchTimeout()
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

	confirmChecker := newConfirmChecker(db, 15*time.Second, 30, 60, service)
	p2p.confirmChecker = confirmChecker
	return p2p
}

func (p2p *P2P) processEvent() {
	for event := range p2p.ch {
		p2p.handler.HandleEvent(event)
	}
}

type watchedHandler struct {
	service *service
	sync.Mutex
	db    *p2pdb
	index *txIndex
	node  *node.BraftNode
	business.Handler
	checkMatchInterval time.Duration
}

func createTx(node *node.BraftNode, op int, info *P2PInfo) interface{} {
	return nil
}

// getP2PInfo p2p交易数据
func getP2PInfo(event *pb.WatchedEvent) *P2PInfo {
	msg := &p2pMsg{}
	msg.Decode(event.GetData())
	p2pMsg := msg.toPBMsg()
	info := &P2PInfo{
		Event: event,
		Msg:   p2pMsg,
		Time:  time.Now().Unix(),
	}
	return info
}

// setWaitConfirm 设置 txID 和 matchedTxID 的双向对应关系
func setWaitConfirm(db *p2pdb, op, chain uint32, txID string) {
	waitConfirmMsg := &WaitConfirmMsg{
		Opration: op,
		Chain:    chain,
		Info:     nil,
	}
	db.setWaitConfirm(txID, waitConfirmMsg)
}

// isConfirmed 判断交易是否已被确认
func isConfirmed(db *p2pdb, txID string) bool {
	waitConfirm := db.getWaitConfirm(txID)
	return waitConfirm != nil && waitConfirm.Info != nil
}

// isMatching 是否在匹配中
func isMatching(db *p2pdb, txID string) bool {
	return db.ExistMatched(txID)
}

// checkP2PInfo check点对点交易是否合法
func (wh *watchedHandler) checkP2PInfo(info *P2PInfo) bool {
	txID := info.GetScTxID()
	if isMatching(wh.db, txID) {
		p2pLogger.Warn("tx is matching or has already been matched", "scTxID", txID)
		return false
	}
	if info.IsExpired() {
		p2pLogger.Warn("tx has already been expired", "scTxID", txID)
		return false
	}
	return true
}

//checkMatchTimeout 交易是否匹配超时
func (wh *watchedHandler) checkMatchTimeout() {
	wh.Lock()
	defer wh.Unlock()
	infos := wh.db.getAllP2PInfos()
	for _, info := range infos {
		// match 超时
		scTxID := info.GetScTxID()
		event := info.Event
		if !isMatching(wh.db, info.GetScTxID()) && !wh.node.IsDone(scTxID) && info.IsExpired() {
			//创建并发送回退交易
			p2pLogger.Debug("match timeout", "scTxID", info.GetScTxID())
			wh.db.setMatchedOne(info.GetScTxID(), "")
			newTx, err := wh.service.createTx(back, info)
			if newTx != nil && err == nil {
				wh.service.sendtoSign(&message.WaitSignMsg{
					Business: event.Business,
					ID:       scTxID,
					ScTxID:   scTxID,
					Event:    event,
					Tx:       newTx,
				})
			}
			setWaitConfirm(wh.db, uint32(back), info.Event.GetTo(), info.GetScTxID())
		}
	}
}

func (wh *watchedHandler) runCheckMatchTimeout() {
	ticker := time.NewTicker(wh.checkMatchInterval).C
	go func() {
		for {
			<-ticker
			wh.checkMatchTimeout()
		}
	}()
}

func (wh *watchedHandler) HandleEvent(event node.BusinessEvent) {
	wh.Lock()
	defer wh.Unlock()
	if watchedEvent, ok := event.(*node.WatchedEvent); ok {
		txEvent := watchedEvent.GetData()
		if event == nil {
			p2pLogger.Error("data is nil", "business", watchedEvent.GetBusiness())
			return
		}
		info := getP2PInfo(txEvent)
		if !wh.checkP2PInfo(info) {
			p2pLogger.Debug("check p2pinfo fail", "scTxID", info.GetScTxID)
			return
		}

		wh.db.setP2PInfo(info)
		// p2pLogger.Debug("add coniditon", "chian", info.Event.From, "addr", info.Msg.SendAddr, "amount", info.Event.Amount)
		wh.index.Add(info)
		//要求匹配的条件
		chain, addr, amount := info.getExchangeInfo()
		// p2pLogger.Debug("search coniditon", "chian", chain, "addr", addr, "amount", amount)
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
				if !wh.checkP2PInfo(matchedInfo) {
					p2pLogger.Debug("check matched p2pinfo fail", "scTxID", info.GetScTxID)
					continue
				}
				//保存已匹配的两笔交易的txid
				wh.db.setMatched(info.GetScTxID(), matchedInfo.GetScTxID())
				//删除索引 防止重复匹配
				wh.index.Del(info)

				infos := []*P2PInfo{info, matchedInfo}
				//创建交易发送签名
				for _, info := range infos {
					newTx, err := wh.service.createTx(confirmed, info)
					if newTx != nil && err == nil {
						scTxID := info.GetScTxID()
						wh.service.sendtoSign(&message.WaitSignMsg{
							Business: watchedEvent.Business,
							ID:       scTxID,
							ScTxID:   scTxID,
							Event:    info.Event,
							Tx:       newTx,
						})

					}
					//设置等待确认
					setWaitConfirm(wh.db, uint32(confirmed), info.Event.GetTo(), info.GetScTxID())
				}

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
	chcker *confirmTimeoutChecker
	business.Handler
	db      *p2pdb
	service *service
}

func (sh *sigenedHandler) HandleEvent(event node.BusinessEvent) {
	if signedEvent, ok := event.(*node.SignedEvent); ok {
		p2pLogger.Info("handle signed")
		signedData := signedEvent.GetData()
		if signedData == nil {
			p2pLogger.Error("signed data is nil")
			return
		}
		txID := signedData.TxID
		if !sh.db.existSendedInfo(txID) && !sh.service.isDone(txID) {
			sh.db.setSendedInfo(&SendedInfo{
				TxId:           signedData.TxID,
				SignTerm:       signedData.Term,
				Chain:          signedData.Chain,
				SignBeforeTxId: signedData.SignBeforeTxID,
			})
			p2pLogger.Debug("receive signedData", "scTxID", signedData.ID)
			//发送交易
			err := sh.service.sendTx(signedData)
			if err != nil {
				p2pLogger.Error("send tx err", "business", signedEvent.Business)
			}
		} else {
			p2pLogger.Debug("already sended", "scTxID", txID)
		}
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
		waitConfirm := handler.db.getWaitConfirm(oldTxID)
		if waitConfirm == nil {
			p2pLogger.Error("never matched tx", "scTxID", oldTxID)
			return
		}
		if waitConfirm.Opration == confirmed { //确认交易 需要等待发起和匹配交易确认

			waitConfirm.Info = info
			handler.db.setWaitConfirm(oldTxID, waitConfirm)

			//先前匹配的交易id
			oldMatchedTxID := handler.db.getMatched(oldTxID)
			oldWaitConfirm := handler.db.getWaitConfirm(oldMatchedTxID)
			if oldWaitConfirm.Info != nil { //与oldTxID匹配的交易已被confirm
				confirmInfos := []*P2PConfirmInfo{info, oldWaitConfirm.Info}
				oldTxIDs := []string{oldTxID, oldMatchedTxID}
				p2pLogger.Debug("get old info", "txIDs", oldTxIDs)
				p2pInfos := handler.db.getP2PInfos(oldTxIDs)
				handler.commitTx(event.GetBusiness(), p2pInfos, confirmInfos)
			} else {
				p2pLogger.Info("wait another tx confirm", "scTxID", oldTxID)
			}

		} else if waitConfirm.Opration == back { //回退交易 commit当前confirmInfo和对应的p2pInfo
			p2pLogger.Debug("hanle confirm back", "scTxID", info.Msg.Id)
			confirmInfos := []*P2PConfirmInfo{info}
			oldTxIDs := []string{oldTxID}
			p2pInfos := handler.db.getP2PInfos(oldTxIDs)
			handler.commitTx(event.GetBusiness(), p2pInfos, confirmInfos)
		} else {
			p2pLogger.Error("oprationtype wrong", "opration", waitConfirm.Opration)
		}

	} else if handler.Successor != nil {
		handler.Successor.HandleEvent(event)
	}
}

type commitHandler struct {
	business.Handler
	db *p2pdb
}

func (ch *commitHandler) HandleEvent(event node.BusinessEvent) {
	if val, ok := event.(*node.CommitedEvent); ok {
		commitedData := val.GetData()
		if commitedData == nil || commitedData.Tx == nil {
			p2pLogger.Error("commit data is nil")
			return
		}
		for _, scTx := range commitedData.Tx.Vin {
			scTxID := scTx.TxID
			ch.db.clear(scTxID)
		}
		fmt.Printf("commitdata:%v", commitedData)
		p2pLogger.Info("handle Commited", "innerTxID")
	} else {
		p2pLogger.Error("could not handle event")
	}
}
