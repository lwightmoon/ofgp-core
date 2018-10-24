package p2p

import (
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/ofgp/common/defines"
	"github.com/ofgp/ofgp-core/log"
	"github.com/ofgp/ofgp-core/message"

	ew "swap/ethwatcher"

	"github.com/golang/protobuf/proto"
	"github.com/ofgp/ofgp-core/business"
	"github.com/ofgp/ofgp-core/node"
	pb "github.com/ofgp/ofgp-core/proto"
)

var p2pLogger = log.New("DEBUG", "node")

// P2P 点对点交换
type P2P struct {
	ch      chan node.BusinessEvent
	handler business.IHandler
}

// NewP2P create p2p
func NewP2P(srv *business.Service, path string) *P2P {
	p2p := &P2P{
		// node: braftNode,
	}
	ldb, _ := node.OpenDbOrDie(path, "p2pdb")

	db := newP2PDB(ldb)
	//向node订阅业务相关事件
	ch := srv.SubScribe(defines.BUSINESS_P2P_SWAP_NAME)
	p2p.ch = ch

	//创建交易匹配索引
	p2pInfos := db.getAllP2PInfos()
	index := newTxIndex()

	index.AddInfos(p2pInfos)
	wh := &watchedHandler{
		db:                 db,
		index:              index,
		checkMatchInterval: time.Duration(1) * time.Second,
		service:            srv,
	}
	// check匹配超时
	wh.runCheckMatchTimeout()

	sh := newSignedHandler(db, srv, 15, 10)
	// sh.runCheck()
	confirmH := newConfirmHandler(db, 180, srv, 10)
	// confirmH.runCheck()
	commitH := &commitHandler{
		db: db,
	}
	wh.SetSuccessor(sh)
	sh.SetSuccessor(confirmH)
	confirmH.SetSuccessor(commitH)
	p2p.handler = wh

	return p2p
}

func (p2p *P2P) Run() {
	go p2p.processEvent()
}
func (p2p *P2P) processEvent() {
	for event := range p2p.ch {
		p2p.handler.HandleEvent(event)
	}
}

type watchedHandler struct {
	service *business.Service
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
	sendAddr := hex.EncodeToString(msg.SendAddr)
	receiveAddr := hex.EncodeToString(msg.ReceiveAddr)
	requireAddr := hex.EncodeToString(msg.RequireAddr)
	p2pLogger.Debug("received event info", "chain", msg.Chain, "scTxID", event.GetTxID(),
		"amount", msg.Amount, "expiretime", msg.ExpiredTime, "sendAddr", sendAddr, "receiveAddr", receiveAddr, "requireAddr", requireAddr)
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
		ScTxId:   txID,
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
		//发起回退交易
		p2pLogger.Warn("tx has already been expired", "scTxID", txID)
		wh.db.setP2PInfo(info)
		wh.sendBackTx(info)
		return false
	}
	return true
}

func (wh *watchedHandler) sendBackTx(info *P2PInfo) {
	//创建并发送回退交易
	wh.db.setMatchedOne(info.GetScTxID(), "")
	req, err := makeCreateTxReq(back, info)
	if req != nil && err == nil {
		signMsg := newWaitToSign(back, info)
		createAndSignMsg := &message.CreateAndSignMsg{
			Req: req,
			Msg: signMsg,
		}
		p2pLogger.Debug("send to sign ", "scTxID", signMsg.ScTxID, "business", signMsg.Business)
		wh.service.SendToSign(createAndSignMsg)
	} else {
		p2pLogger.Error("create tx err", "err", err, "scTxID", info.Event.GetTxID())
	}
	setWaitConfirm(wh.db, uint32(back), info.Event.GetTo(), info.GetScTxID())
}

//checkMatchTimeout 交易是否匹配超时
func (wh *watchedHandler) checkMatchTimeout() {
	wh.Lock()
	defer wh.Unlock()
	infos := wh.db.getAllP2PInfos()
	for _, info := range infos {
		// match 超时
		scTxID := info.GetScTxID()
		if !isMatching(wh.db, info.GetScTxID()) && !wh.node.IsDone(scTxID) && info.IsExpired() {
			//创建并发送回退交易
			p2pLogger.Debug("match timeout", "scTxID", info.GetScTxID())
			wh.sendBackTx(info)
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

func newWaitToSign(op uint8, info *P2PInfo) *message.WaitSignMsg {
	var waitToSign *message.WaitSignMsg
	var chain uint8
	var addr []byte
	var token uint32
	switch op {
	case confirmed:
		chain = uint8(info.Msg.Chain)
		addr = info.Msg.ReceiveAddr
		token = info.Msg.TokenId
	case back:
		chain = uint8(info.Event.GetFrom())
		addr = info.Msg.SendAddr
		token = info.Msg.FromToken
	default:
		p2pLogger.Error("create signreq op err", "scTxID", info.Event.GetTxID())
		return nil
	}
	switch chain {
	case defines.CHAIN_CODE_BTC:
		fallthrough
	case defines.CHAIN_CODE_BCH:
		recharge := &pb.BtcRecharge{
			Amount: info.Msg.Amount,
			Addr:   addr,
		}
		rechargeData, _ := proto.Marshal(recharge)
		waitToSign = &message.WaitSignMsg{
			Business: info.Event.Business,
			ID:       info.Event.GetTxID(),
			ScTxID:   info.Event.GetTxID(),
			Event:    info.Event,
			Recharge: rechargeData,
		}
	case defines.CHAIN_CODE_ETH:
		recharge := &pb.EthRecharge{
			Addr:     addr,
			Amount:   info.Msg.Amount,
			TokenTo:  token,
			Method:   ew.VOTE_METHOD_MATCHSWAP,
			Proposal: info.Event.GetTxID(),
		}
		rechargeData, _ := proto.Marshal(recharge)
		waitToSign = &message.WaitSignMsg{
			Business: info.Event.Business,
			ID:       info.Event.GetTxID(),
			ScTxID:   info.Event.GetTxID(),
			Event:    info.Event,
			Recharge: rechargeData,
		}

	}
	return waitToSign
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
		if wh.db.getP2PInfo(txEvent.GetTxID()) != nil {
			p2pLogger.Warn("tx already received", "scTxID", txEvent.GetTxID())
			return
		}
		info := getP2PInfo(txEvent)

		if !wh.checkP2PInfo(info) {
			p2pLogger.Debug("check p2pinfo fail", "scTxID", info.GetScTxID())
			return
		}

		wh.db.setP2PInfo(info)
		// p2pLogger.Debug("add coniditon", "chian", info.Event.From, "addr", info.Msg.SendAddr, "amount", info.Event.Amount)
		wh.index.Add(info)
		//要求匹配的条件
		chain, addr, amount := info.getExchangeInfo()
		p2pLogger.Debug("search coniditon", "chian", chain, "addr", addr, "amount", amount)
		// 使用要求的数据匹配交易数据
		txIDs := wh.index.GetTxID(chain, addr, amount)

		if txIDs != nil {
			for _, txID := range txIDs {
				p2pLogger.Debug("matchedTxID", "txID", txID, "scTxID", txEvent.GetTxID())
				matchedInfo := wh.db.getP2PInfo(txID)
				if matchedInfo == nil {
					p2pLogger.Error("get p2pInfo nil", "business", event.GetBusiness(), "scTxID", txEvent.GetTxID())
					continue
				}
				if !wh.checkP2PInfo(matchedInfo) {
					p2pLogger.Debug("check matched p2pinfo fail", "scTxID", info.GetScTxID())
					continue
				}
				//保存已匹配的两笔交易的txid
				wh.db.setMatched(info.GetScTxID(), matchedInfo.GetScTxID())
				//删除索引 防止重复匹配
				wh.index.Del(info)
				wh.index.Del(matchedInfo)

				infos := []*P2PInfo{info, matchedInfo}
				//创建交易发送签名
				for _, info := range infos {
					req, err := makeCreateTxReq(confirmed, info)
					if req != nil && err == nil {
						signMsg := newWaitToSign(confirmed, info)
						createAndSignMsg := &message.CreateAndSignMsg{
							Req: req,
							Msg: signMsg,
						}
						wh.service.SendToSign(createAndSignMsg)
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

type signedHandler struct {
	business.Handler
	db          *p2pdb
	service     *business.Service
	signFailTxs map[string]*WaitConfirmMsg //签名失败
	signTimeout int64
	sync.Mutex
	interval time.Duration
}

func newSignedHandler(db *p2pdb, service *business.Service, signTimeout int64, interval time.Duration) *signedHandler {
	return &signedHandler{
		db:          db,
		service:     service,
		signFailTxs: make(map[string]*WaitConfirmMsg),
		signTimeout: signTimeout,
		interval:    interval,
	}
}

func (sh *signedHandler) HandleEvent(event node.BusinessEvent) {
	if signedEvent, ok := event.(*node.SignedEvent); ok {
		p2pLogger.Info("handle signed")
		signedData := signedEvent.GetData()
		if signedData == nil {
			p2pLogger.Error("signed data is nil")
			return
		}
		txID := signedData.TxID
		sh.Lock()
		if !sh.db.existSendedInfo(txID) && !sh.service.IsDone(txID) {
			p2pLogger.Debug("receive signedData", "scTxID", signedData.ID)
			//发送交易
			err := sh.service.SendTx(signedData)
			if err != nil {
				p2pLogger.Error("send tx err", "err", err, "scTxID", signedData.ID, "business", signedEvent.Business)
			} else {
				sh.db.setSendedInfo(&SendedInfo{
					TxId: signedData.TxID,
					// SignTerm:       signedData.Term,
					Chain: signedData.Chain,
					// SignBeforeTxId: signedData.SignBeforeTxID,
				})
			}
		} else {
			p2pLogger.Debug("already sended", "scTxID", txID)
		}
		sh.Unlock()
	} else if sh.Successor != nil {
		sh.Successor.HandleEvent(event)
	}
}

type confirmHandler struct {
	sync.Mutex
	db *p2pdb
	business.Handler
	confirmTolerance int64
	service          *business.Service
	interval         time.Duration
}

func newConfirmHandler(db *p2pdb, confirmTolerance int64, service *business.Service, interval time.Duration) *confirmHandler {
	return &confirmHandler{
		db:               db,
		confirmTolerance: confirmTolerance,
		service:          service,
		interval:         interval,
	}
}

// getPubTxFromInfo 获取网关tx vin
func getPubTxFromInfo(info *P2PInfo) *pb.PublicTx {
	if info == nil {
		return nil
	}
	event := info.GetEvent()
	chain := uint8(event.GetTo())
	var rechargeData []byte
	switch chain {
	case defines.CHAIN_CODE_BTC:
		fallthrough
	case defines.CHAIN_CODE_BCH:
		recharge := &pb.BtcRecharge{
			Addr:   info.GetMsg().GetReceiveAddr(),
			Amount: info.GetMsg().GetAmount(),
		}
		rechargeData, _ = proto.Marshal(recharge)
	case defines.CHAIN_CODE_ETH:
		recharge := &pb.EthRecharge{
			Addr:   info.GetMsg().GetReceiveAddr(),
			Amount: info.GetMsg().GetAmount(),
		}
		rechargeData, _ = proto.Marshal(recharge)
	}
	pubTx := &pb.PublicTx{
		Chain:    event.GetFrom(),
		TxID:     event.GetTxID(),
		Amount:   int64(event.GetAmount()),
		Data:     event.GetData(),
		Recharge: rechargeData,
		Code:     info.GetMsg().GetFromToken(),
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

func getPubTxVout(info *P2PInfo, confirmInfo *P2PConfirmInfo) *pb.PublicTx {
	if info == nil || confirmInfo == nil {
		p2pLogger.Error("get pubtx vout info confirm info is nil")
		return nil
	}
	event := confirmInfo.GetEvent()
	var code uint32
	if msg := info.GetMsg(); msg != nil {
		code = msg.GetTokenId()
	} else {
		panic("info msg is nil")
	}
	pubTx := &pb.PublicTx{
		Chain:  event.GetTo(),
		TxID:   event.GetTxID(),
		Amount: int64(event.GetAmount()),
		Data:   event.GetData(),
		Code:   code,
	}
	return pubTx
}

func getPubTxFromConfirmInfo(info *P2PConfirmInfo) *pb.PublicTx {
	if info == nil {
		return nil
	}
	event := info.GetEvent()
	pubTx := &pb.PublicTx{
		Chain:  event.GetTo(),
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
	handler.service.CommitTx(dgwTx)
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

func (handler *confirmHandler) getConfirmTimeout(chain uint32) int64 {
	chainCmp := uint8(chain)
	switch chainCmp {
	case defines.CHAIN_CODE_BCH:
		fallthrough
	case defines.CHAIN_CODE_BTC:
		return int64(node.BchConfirms)*60*10 + handler.confirmTolerance
	case defines.CHAIN_CODE_ETH:
		return int64(node.BchConfirms)*15 + handler.confirmTolerance
	default:
		return handler.confirmTolerance
	}

}

func createVinAndVout(info *P2PInfo, confirmInfo *P2PConfirmInfo) (inTx, outTx *pb.PublicTx) {
	if info == nil || confirmInfo == nil {
		p2pLogger.Error("info confirm info can't be nil")
		return
	}
	inTx = getPubTxFromInfo(info)
	outTx = getPubTxVout(info, confirmInfo)
	return
}

func (handler *confirmHandler) HandleEvent(event node.BusinessEvent) {
	if confirmedEvent, ok := event.(*node.ConfirmEvent); ok {
		txEvent := confirmedEvent.GetData()
		if event == nil {
			p2pLogger.Error("confirm data is nil")
			return
		}
		//交易确认info
		nowConfirmInfo := getP2PConfirmInfo(txEvent)

		//之前的交易id
		oldTxID := txEvent.GetProposal()
		p2pLogger.Info("handle confirm", "scTxID", oldTxID)
		waitConfirm := handler.db.getWaitConfirm(oldTxID)
		if waitConfirm == nil {
			p2pLogger.Error("never matched tx", "scTxID", oldTxID)
			return
		}
		if waitConfirm.Opration == confirmed { //确认交易 需要等待发起和匹配交易确认

			waitConfirm.Info = nowConfirmInfo
			handler.db.setWaitConfirm(oldTxID, waitConfirm)

			//先前匹配的交易id
			oldMatchedTxID := handler.db.getMatched(oldTxID)
			if oldMatchedTxID == "" {
				p2pLogger.Error("can not find matched", "oldTxID", oldTxID)
				return
			}
			matchedWaitConfirm := handler.db.getWaitConfirm(oldMatchedTxID)
			if matchedWaitConfirm == nil {
				p2pLogger.Error("matched tx never send to sign", "oldMatchTxID", oldMatchedTxID)
				return
			}
			if matchedConfirmInfo := matchedWaitConfirm.Info; matchedConfirmInfo != nil { //与oldTxID匹配的交易已被confirm
				/*
					confirmInfos := []*P2PConfirmInfo{info, oldWaitConfirm.Info}
					oldTxIDs := []string{oldTxID, oldMatchedTxID}
					p2pLogger.Debug("get old info", "txIDs", oldTxIDs)
					p2pInfos := handler.db.getP2PInfos(oldTxIDs)
					handler.commitTx(event.GetBusiness(), p2pInfos, confirmInfos)
				*/
				nowRequireInfo := handler.db.getP2PInfo(oldTxID)
				vinNow, voutNow := createVinAndVout(nowRequireInfo, nowConfirmInfo)
				matchedRequireInfo := handler.db.getP2PInfo(oldMatchedTxID)
				vinMatched, voutMatched := createVinAndVout(matchedRequireInfo, matchedConfirmInfo)
				dgwTx := &pb.Transaction{
					Business: event.GetBusiness(),
					Vin:      []*pb.PublicTx{vinMatched, vinNow},
					Vout:     []*pb.PublicTx{voutMatched, voutNow},
					Time:     time.Now().Unix(),
				}
				dgwTx.UpdateId()
				txJSON, _ := json.Marshal(dgwTx)
				p2pLogger.Debug("commit data", "data", string(txJSON))
				handler.service.CommitTx(dgwTx)
			} else {
				p2pLogger.Info("wait another tx confirm", "scTxID", oldTxID)
			}

		} else if waitConfirm.Opration == back { //回退交易 commit当前confirmInfo和对应的p2pInfo
			p2pLogger.Debug("hanle confirm back", "scTxID", oldTxID)
			// confirmInfos := []*P2PConfirmInfo{nowConfirmInfo}
			// oldTxIDs := []string{oldTxID}
			// p2pInfos := handler.db.getP2PInfos(oldTxIDs)
			// handler.commitTx(event.GetBusiness(), p2pInfos, confirmInfos)
			nowRequireInfo := handler.db.getP2PInfo(oldTxID)
			vin, vout := createVinAndVout(nowRequireInfo, nowConfirmInfo)
			dgwTx := &pb.Transaction{
				Business: event.GetBusiness(),
				Vin:      []*pb.PublicTx{vin},
				Vout:     []*pb.PublicTx{vout},
				Time:     time.Now().Unix(),
			}
			dgwTx.UpdateId()
			txJSON, _ := json.Marshal(dgwTx)
			p2pLogger.Debug("commit data", "data", string(txJSON))
			handler.service.CommitTx(dgwTx)

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
		var matchedTxs []string
		for _, scTx := range commitedData.Tx.Vin {
			scTxID := scTx.TxID
			matchedTxs = append(matchedTxs, scTxID)
			ch.db.clear(scTxID)
		}
		p2pLogger.Info("commited txID", "matched", matchedTxs, "business", val.Business)
		// p2pLogger.Info("handle Commited", "innerTxID", commitedData.Tx.TxID)
	} else {
		p2pLogger.Error("could not handle event")
	}
}
