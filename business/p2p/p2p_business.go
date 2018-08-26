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
	db   *p2pdb
	node *node.BraftNode
	business.Handler
}

// signTx 签名交易
func (wh *watchedHandler) sendToSignTx(tx *P2PTx, seqID []byte) {
	wh.db.setP2PTx(tx, seqID)
	//todo 创建交易，并交给网关签名
	p2pLogger.Debug("create tx and send to sign")
}
func (wh *watchedHandler) HandleEvent(event node.BusinessEvent) {
	if watchedEvent, ok := event.(*node.WatchedEvent); ok {
		event := watchedEvent.GetData()
		if event == nil {
			p2pLogger.Error("data is nil", "business", watchedEvent.GetBusiness())
			return
		}
		msg := &p2pMsg{}
		msg.Decode(event.GetData())
		p2pMsg := msg.toPBMsg()
		seqID := msg.SeqID

		p2pInfo := &P2PInfo{
			Event: event,
			Msg:   p2pMsg,
		}
		if tx := wh.db.getP2PTx(seqID); tx == nil {
			wh.Lock()
			tx = wh.db.getP2PTx(seqID)
			if tx == nil {
				tx = &P2PTx{
					SeqId: seqID,
				}
				tx.AddInfo(p2pInfo)
				wh.db.setP2PTx(tx, seqID)
			} else if !tx.Finished { //match tx
				tx.AddInfo(p2pInfo)
				tx.SetFinished()
				wh.sendToSignTx(tx, seqID)
			} else {
				p2pLogger.Debug("already finished")
			}
			wh.Unlock()
		} else if !tx.Finished { //match tx
			tx.AddInfo(p2pInfo)
			tx.SetFinished()
			wh.sendToSignTx(tx, seqID)
		} else {
			p2pLogger.Debug("already finished")
		}
		//保存交易id与seqID对应关系
		wh.db.setTxSeqIDMap(event.GetTxID(), seqID)
		p2pLogger.Info("handle watched", "scTxID", event.GetTxID())

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
func getVin(p2pTx *P2PTx) []*pb.PublicTx {
	if p2pTx == nil {
		return nil
	}
	pubtxs := make([]*pb.PublicTx, 0)
	pubtx := getPubTxFromInfo(p2pTx.Initiator)
	if pubtx != nil {
		pubtxs = append(pubtxs, pubtx)
	}
	pubtx = getPubTxFromInfo(p2pTx.Matcher)
	if pubtx != nil {
		pubtxs = append(pubtxs, pubtx)
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

func getVout(p2pTx *P2PNewTx) []*pb.PublicTx {
	if p2pTx == nil {
		return nil
	}
	pubtxs := make([]*pb.PublicTx, 0)
	pubtx := getPubTxFromConfirmInfo(p2pTx.Initiator)
	if pubtx != nil {
		pubtxs = append(pubtxs, pubtx)
	}
	pubtx = getPubTxFromConfirmInfo(p2pTx.Matcher)
	if pubtx != nil {
		pubtxs = append(pubtxs, pubtx)
	}
	return pubtxs
}

// createDGWTx 创建网关交易
func createDGWTx(business string, p2pTx *P2PTx, p2pNewTx *P2PNewTx) *pb.Transaction {
	tx := &pb.Transaction{
		Business: business,
		Vin:      getVin(p2pTx),
		Vout:     getVout(p2pNewTx),
		Time:     time.Now().Unix(),
	}
	tx.UpdateId()
	return tx
}

// commitTx commit点对点交易
func (handler *confirmHandler) commitTx(business string, p2pNewTx *P2PNewTx, seqID []byte) {
	p2pTx := handler.db.getP2PTx(seqID)
	dgwTx := createDGWTx(business, p2pTx, p2pNewTx)
	p2pLogger.Debug("commit innter tx", "innerTxID", dgwTx.TxID.ToText(), "initial", p2pTx.Initiator.GetScTxID(), "matched", p2pTx.Matcher.GetScTxID(),
		"initial_new", p2pNewTx.Initiator.GetTxID(), "match_new", p2pNewTx.Matcher.GetTxID())
	handler.node.Commit(dgwTx)
}

func setConfirmInfo(newTx *P2PNewTx, scTxID string, pre *P2PTx, info *P2PConfirmInfo) {
	if scTxID == pre.Initiator.GetScTxID() {
		newTx.Initiator = info
	} else {
		newTx.Matcher = info
	}
}
func (handler *confirmHandler) HandleEvent(event node.BusinessEvent) {
	if confirmedEvent, ok := event.(*node.ConfirmEvent); ok {

		event := confirmedEvent.GetData()
		if event == nil {
			p2pLogger.Error("confirm data is nil")
			return
		}

		msg := &p2pMsgConfirmed{}
		msg.Decode(event.GetData())
		// 业务相关msg
		pbMsg := msg.toPBMsg()
		//交易序列号
		seqID := handler.db.getTxSeqID(pbMsg.Id)
		if seqID == nil || len(seqID) == 0 {
			p2pLogger.Warn("get seqID nil", "scTxID", pbMsg.Id)
			return
		}

		//交易确认info
		confirmInfo := &P2PConfirmInfo{
			Event: event,
			Msg:   pbMsg,
		}
		p2pLogger.Info("handle confirm", "scTxID", pbMsg.Id)

		//匹配的交易
		watchedP2PTx := handler.db.getP2PTx(seqID)

		if msg.Opration == confirmed { //确认交易 需要等待发起和匹配交易确认
			if seqID == nil || len(seqID) == 0 {
				p2pLogger.Error("receive msg haven'n received", "scTxID", pbMsg.Id)
				return
			}
			newTx := handler.db.getP2pNewTx(seqID)
			if newTx == nil { //set confirm
				handler.Lock()
				if newTx = handler.db.getP2pNewTx(seqID); newTx == nil {
					newTx = newP2PNewTx(seqID)
					p2pLogger.Debug("id compare", "pbmsg", pbMsg.Id, "scTxID", watchedP2PTx.Initiator.GetScTxID())
					setConfirmInfo(newTx, pbMsg.Id, watchedP2PTx, confirmInfo)
					handler.db.setP2PNewTx(newTx, seqID)
				} else if !newTx.Finished { //commit
					newTx = newP2PNewTx(seqID)
					setConfirmInfo(newTx, pbMsg.Id, watchedP2PTx, confirmInfo)
					handler.commitTx(confirmedEvent.Business, newTx, seqID)
				} else {
					p2pLogger.Debug("tx already commited")
				}
				handler.Unlock()
			} else if !newTx.Finished { //commit
				setConfirmInfo(newTx, pbMsg.Id, watchedP2PTx, confirmInfo)
				handler.commitTx(confirmedEvent.Business, newTx, seqID)
			} else {
				p2pLogger.Debug("tx already commited")
			}
		} else { //回退交易 只须commit即可
			newTx := newP2PNewTx(seqID)
			setConfirmInfo(newTx, pbMsg.Id, watchedP2PTx, confirmInfo)
			handler.db.setP2PNewTx(newTx, seqID)
			handler.commitTx(confirmedEvent.Business, newTx, seqID)
		}

	} else if handler.Successor != nil {
		handler.Successor.HandleEvent(event)
	}
}

func newP2PNewTx(seqID []byte) *P2PNewTx {
	return &P2PNewTx{
		SeqId: seqID,
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
