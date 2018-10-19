package mint

import (
	"encoding/json"
	"errors"
	"time"

	proto "github.com/golang/protobuf/proto"
	"github.com/ofgp/common/defines"
	"github.com/ofgp/ofgp-core/log"
	"github.com/ofgp/ofgp-core/message"

	"github.com/ofgp/ofgp-core/business"
	"github.com/ofgp/ofgp-core/node"
	pb "github.com/ofgp/ofgp-core/proto"
)

var mintLogger = log.New("DEBUG", "node")

// Processer 处理铸币熔币
type Processer struct {
	ch      chan node.BusinessEvent
	handler business.IHandler
}

// NewProcesser 创建铸币熔币处理
func NewProcesser(srv business.IService, path string) *Processer {
	ldb, _ := business.OpenDbOrDie(path, "mint")

	// mintdb
	db := newMintDB(ldb)
	// eventchannel todo
	eventCh := srv.SubScribe("")

	watchedHd := newWatchedHandler(db, srv)
	signedHd := newSignedHandler(db, srv)
	confirmedHd := newConfirmHandler(db, srv)
	commitedHd := newCommitHandler(db)

	watchedHd.SetSuccessor(signedHd)
	signedHd.SetSuccessor(confirmedHd)
	confirmedHd.SetSuccessor(commitedHd)

	p := &Processer{
		ch:      eventCh,
		handler: watchedHd,
	}
	return p
}

// Run start
func (p *Processer) Run() {
	go p.processEvent()
}

func (p *Processer) processEvent() {
	for event := range p.ch {
		p.handler.HandleEvent(event)
	}
}

type watchedHandler struct {
	business.Handler
	db      *mintDB
	service business.IService
}

func newWatchedHandler(db *mintDB, service business.IService) *watchedHandler {
	return &watchedHandler{
		db:      db,
		service: service,
	}
}

func makeCreateTxReq(info *MintInfo) (message.CreateReq, error) {
	event := info.GetEvent()
	require := info.GetReq()

	chain := uint8(event.GetTo())

	var txReq message.CreateReq
	switch chain {
	case defines.CHAIN_CODE_BCH:
		fallthrough
	case defines.CHAIN_CODE_BTC:
		txReq = &node.BaseCreateReq{
			Chain:  uint32(chain),
			ID:     event.GetTxID(),
			Addr:   require.Receiver,
			Amount: event.GetAmount(),
		}
	case defines.CHAIN_CODE_ETH:
		ethReq := &node.EthCreateReq{}
		ethReq.Chain = uint32(chain)
		ethReq.ID = event.GetTxID()
		ethReq.Addr = require.Receiver
		ethReq.Amount = event.GetAmount()
		ethReq.TokenTo = require.TokenTo
		txReq = ethReq
	default:
		mintLogger.Error("chain type err")
		return nil, errors.New("chain type err")
	}
	return txReq, nil
}

func makeSignMsg(info *MintInfo) *message.WaitSignMsg {
	event := info.GetEvent()
	recharge := getRecharge(info)
	signMsg := &message.WaitSignMsg{
		Business: event.GetBusiness(),
		ID:       event.GetTxID(),
		ScTxID:   event.GetTxID(),
		Event:    event,
		Recharge: recharge,
	}
	return signMsg
}

func (handler *watchedHandler) HandleEvent(event node.BusinessEvent) {
	if val, ok := event.(*node.WatchedEvent); ok {
		watchedEvent := val.GetData()
		if watchedEvent == nil {
			mintLogger.Error("watchedEvent is nil", "business", event.GetBusiness())
			return
		}
		scTxID := watchedEvent.GetTxID()
		if handler.db.existMintInfo(scTxID) {
			mintLogger.Warn("already received mint", "scTxID", scTxID)
			return
		}

		mintReq := &MintRequire{}
		mintReq.decode(watchedEvent.GetData())

		mintInfo := &MintInfo{
			Event: watchedEvent,
			Req:   mintReq,
		}
		handler.db.setMintInfo(mintInfo)

		txReq, err := makeCreateTxReq(mintInfo)
		if err != nil {
			mintLogger.Error("carete txReq err", "err", err, "scTxID", watchedEvent.GetTxID())
			return
		}
		signReq := makeSignMsg(mintInfo)

		// 发送请求到共识层
		createAndSignMsg := &message.CreateAndSignMsg{
			Req: txReq,
			Msg: signReq,
		}
		handler.service.SendToSign(createAndSignMsg)

	} else if handler.Successor != nil {
		handler.HandleEvent(event)
	}
}

type signedHandler struct {
	business.Handler
	db      *mintDB
	service business.IService
}

func newSignedHandler(db *mintDB, srv business.IService) *signedHandler {
	return &signedHandler{
		db:      db,
		service: srv,
	}
}

func (hd *signedHandler) HandleEvent(event node.BusinessEvent) {
	if val, ok := event.(*node.SignedEvent); ok {
		signedData := val.GetData()
		if signedData == nil {
			mintLogger.Error("signedData is nil", "business", event.GetBusiness())
			return
		}
		txID := signedData.TxID
		if !hd.db.isSended(txID) && !hd.service.IsDone(txID) {
			mintLogger.Debug("receive signedData", "scTxID", signedData.ID)
			//发送交易
			err := hd.service.SendTx(signedData)
			if err != nil {
				mintLogger.Error("send tx err", "err", err, "scTxID", signedData.ID, "business", event.GetBusiness())
			} else {
				hd.db.setSended(txID)
			}
		} else {
			mintLogger.Debug("already sended", "scTxID", txID)
		}

	} else if hd.Successor != nil {
		hd.Successor.HandleEvent(event)
	}
}

type confirmedHandler struct {
	db *mintDB
	business.Handler
	service business.IService
}

func newConfirmHandler(db *mintDB, srv business.IService) *confirmedHandler {
	return &confirmedHandler{
		db:      db,
		service: srv,
	}
}

func getRecharge(info *MintInfo) []byte {
	var data []byte
	event := info.GetEvent()
	req := info.GetReq()
	chain := uint8(event.GetTo())
	switch chain {
	case defines.CHAIN_CODE_BTC:
		fallthrough
	case defines.CHAIN_CODE_BCH:
		recharge := &pb.BtcRecharge{
			Amount: event.GetAmount(),
			Addr:   req.GetReceiver(),
		}
		data, _ = proto.Marshal(recharge)
	case defines.CHAIN_CODE_ETH:
		recharge := &pb.EthRecharge{
			Addr:     req.GetReceiver(),
			Amount:   event.GetAmount(),
			TokenTo:  req.TokenTo,
			Method:   "",
			Proposal: event.GetTxID(),
		}
		data, _ = proto.Marshal(recharge)
	default:
		mintLogger.Error("getrecharge type err")
	}
	return data
}
func getVin(info *MintInfo) *pb.PublicTx {
	event := info.GetEvent()
	req := info.GetReq()
	recharge := getRecharge(info)
	pubTx := &pb.PublicTx{
		Chain:    event.GetFrom(),
		TxID:     event.GetTxID(),
		Amount:   int64(event.GetAmount()),
		Data:     event.GetData(),
		Recharge: recharge,
		Code:     req.TokenFrom,
	}
	return pubTx
}

func getVout(confirmEvent *pb.WatchedEvent, info *MintInfo) *pb.PublicTx {
	pubTx := &pb.PublicTx{
		Chain:  confirmEvent.GetTo(),
		TxID:   confirmEvent.GetTxID(),
		Amount: int64(confirmEvent.GetAmount()),
		Data:   confirmEvent.GetData(),
		Code:   info.GetReq().GetTokenTo(),
	}
	return pubTx
}

func (hd *confirmedHandler) HandleEvent(event node.BusinessEvent) {
	if confirmEvent, ok := event.(*node.ConfirmEvent); ok {
		watchedEvent := confirmEvent.GetData()
		if watchedEvent == nil {
			mintLogger.Error("watched Event is nil", "business", event.GetBusiness())
			return
		}
		scTxID := watchedEvent.GetProposal()
		if scTxID == "" {
			mintLogger.Error("confirm event has not proposal")
			return
		}
		mintInfo := hd.db.getMintInfo(scTxID)
		vin := getVin(mintInfo)
		vout := getVout(watchedEvent, mintInfo)

		dgwTx := &pb.Transaction{
			Business: event.GetBusiness(),
			Vin:      []*pb.PublicTx{vin},
			Vout:     []*pb.PublicTx{vout},
			Time:     time.Now().Unix(),
		}
		dgwTx.UpdateId()
		txJSON, _ := json.Marshal(dgwTx)
		mintLogger.Debug("commit data", "data", string(txJSON))
		hd.service.CommitTx(dgwTx)
	} else if hd.Successor != nil {
		hd.Successor.HandleEvent(event)
	}
}

type commitedHandler struct {
	business.Handler
	db *mintDB
}

func newCommitHandler(db *mintDB) *commitedHandler {
	return &commitedHandler{
		db: db,
	}
}

func (hd *commitedHandler) HandleEvent(event node.BusinessEvent) {
	if commitedEvent, ok := event.(*node.CommitedEvent); ok {
		data := commitedEvent.GetData()
		if data == nil || data.Tx == nil {
			mintLogger.Error("commit data is nil", "business", event.GetBusiness())
			return
		}
		var txIDs []string
		for _, scTx := range data.Tx.Vin {
			scTxID := scTx.TxID
			txIDs = append(txIDs, scTxID)
			hd.db.clear(scTxID)
		}
		mintLogger.Info("commitedTxID", "scTxIDs", txIDs)
	} else {
		mintLogger.Error("could not handle event")
	}
}
