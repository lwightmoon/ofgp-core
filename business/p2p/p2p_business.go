package p2p

import (
	"fmt"
	"sync"

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

// createDGWTx 创建网关交易
func createDGWTx(p2pTx *P2PTx, p2pNewTx *P2PNewTx) *pb.Transaction {
	tx := &pb.Transaction{}
	tx.UpdateId()
	return tx
}

// commitTx commit点对点交易
func (handler *confirmHandler) commitTx(seqID []byte) {
	p2pTx := handler.db.getP2PTx(seqID)
	p2pNewTx := handler.db.getP2pNewTx(seqID)
	dgwTx := createDGWTx(p2pTx, p2pNewTx)
	p2pLogger.Debug("commit innter tx", "innerTxID", dgwTx.TxID.ToText(), "initial", p2pTx.Initiator.GetScTxID(), "matched", p2pTx.Matcher.GetScTxID(),
		"initial_new", p2pNewTx.Initiator.GetTxID(), "match_new", p2pNewTx.Matcher.GetTxID())
	handler.node.Commit(dgwTx)
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
		pbMsg := msg.toPBMsg()
		seqID := handler.db.getTxSeqID(pbMsg.Id)
		if msg.Opration == confirmed { //确认交易 需要等待发起和匹配交易确认
			p2pLogger.Info("handle confirm", "scTxID", pbMsg.Id)
			watchedP2PTx := handler.db.getP2PTx(seqID)
			if seqID == nil || len(seqID) == 0 {
				p2pLogger.Error("receive msg haven'n received", "scTxID", pbMsg.Id)
				return
			}
			newTx := handler.db.getP2pNewTx(seqID)
			if newTx == nil { //set confirm
				handler.Lock()
				if newTx = handler.db.getP2pNewTx(seqID); newTx == nil {
					newTx.SeqId = seqID
					confirmInfo := &P2PConfirmInfo{
						Event: event,
						Msg:   pbMsg,
					}
					if pbMsg.Id == watchedP2PTx.Initiator.GetScTxID() {
						newTx.Initiator = confirmInfo
					} else {
						newTx.Matcher = confirmInfo
					}
					handler.db.setP2PNewTx(newTx, seqID)
				} else { //commit
					handler.commitTx(seqID)
				}
				handler.Unlock()
			} else { //commit
				handler.commitTx(seqID)
			}
		} else { //回退交易

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
