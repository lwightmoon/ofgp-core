package primitives

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/dgwdb"
	"github.com/ofgp/ofgp-core/message"
	pb "github.com/ofgp/ofgp-core/proto"
	"github.com/ofgp/ofgp-core/util"
	"github.com/ofgp/ofgp-core/util/assert"
)

const (
	// 交易的过期时间，过期直接从mempool里面删除
	txTTL = 10 * 60 * time.Second
	// 交易的处理超时时间，超时会发起accuse
	txWaitingTolerance        = 60 * 3 * time.Second
	heartbeatInterval         = 10 * time.Second
	defaultTxWaitingTolerance = 60 * time.Second  //默认交易超时时间
	maxTxWaitingTolerance     = 300 * time.Second //交易最大超时时间
)

const (
	NotExist = iota
	Valid
	Invalid
	CheckChain
)

type addTxsRequest struct {
	txs          []*pb.Transaction
	notAddedChan chan []int
}

func makeAddTxsRequest(txs []*pb.Transaction) addTxsRequest {
	return addTxsRequest{txs, make(chan []int, 1)}
}

// TxQueryResult 保存搜索结果
type TxQueryResult struct {
	Tx      *pb.Transaction
	Height  int64
	BlockID *crypto.Digest256
}

type blockCommitted struct {
	txs    []*pb.Transaction
	height int64
}

func makeBlockCommitted(newBlock *pb.BlockPack) blockCommitted {
	return blockCommitted{newBlock.Block().Txs, newBlock.Height()}
}

// txWithTimeMs 待打包交易
type txWithTimeMs struct {
	tx                 *pb.Transaction
	genTs              time.Time
	calcOverdueTs      time.Time
	txWaitingTolerance time.Duration
}

func (t *txWithTimeMs) IsOverdue() bool {
	if t.txWaitingTolerance == 0 {
		t.txWaitingTolerance = defaultTxWaitingTolerance
	}
	return time.Now().After(t.calcOverdueTs.Add(txWaitingTolerance))
}

func (t *txWithTimeMs) IsOutdate() bool {
	return time.Now().After(t.genTs.Add(txTTL))
}

// resetWaitingTolerance 设置超时时间为原先的两倍直到大于最大超时时间
func (t *txWithTimeMs) resetWaitingTolerance() {
	tempTolerance := 2 * t.txWaitingTolerance
	if tempTolerance > maxTxWaitingTolerance {
		tempTolerance = maxTxWaitingTolerance
	}
	t.txWaitingTolerance = tempTolerance
	t.calcOverdueTs = time.Now()
}

func newSignMsgWithtimeMs(msg *message.WaitSignMsg) *signMsgWithtimeMs {
	return &signMsgWithtimeMs{
		msg:                msg,
		genTs:              time.Now(),
		calcOverdueTs:      time.Now(),
		txWaitingTolerance: defaultTxWaitingTolerance,
	}
}

type signMsgWithtimeMs struct {
	msg                *message.WaitSignMsg
	genTs              time.Time
	calcOverdueTs      time.Time
	txWaitingTolerance time.Duration
}

func (msg *signMsgWithtimeMs) IsOverdue() bool {
	if msg.txWaitingTolerance == 0 {
		msg.txWaitingTolerance = defaultTxWaitingTolerance
	}
	return time.Now().After(msg.calcOverdueTs.Add(txWaitingTolerance))
}

func (msg *signMsgWithtimeMs) IsOutdate() bool {
	return time.Now().After(msg.genTs.Add(txTTL))
}

// resetWaitingTolerance 设置超时时间为原先的两倍直到大于最大超时时间
func (msg *signMsgWithtimeMs) resetWaitingTolerance() {
	tempTolerance := 2 * msg.txWaitingTolerance
	if tempTolerance > maxTxWaitingTolerance {
		tempTolerance = maxTxWaitingTolerance
	}
	msg.txWaitingTolerance = tempTolerance
	msg.calcOverdueTs = time.Now()
}

// WatchedTxInfo watcher监听到的交易信息
type WatchedTxInfo struct {
	Tx                 *pb.WatchedTxInfo
	genTs              time.Time
	calcOverdueTs      time.Time     //交易创建时间
	txWaitingTolerance time.Duration //交易超时时间
}

func (t *WatchedTxInfo) isOverdue() bool {
	if t.txWaitingTolerance == 0 {
		t.txWaitingTolerance = defaultTxWaitingTolerance
	}
	return time.Now().After(t.calcOverdueTs.Add(t.txWaitingTolerance))

}

// resetWaitingTolerance 设置超时时间为原先的两倍直到大于最大超时时间
func (t *WatchedTxInfo) resetWaitingTolerance() {
	tempTolerance := 2 * t.txWaitingTolerance
	if tempTolerance > maxTxWaitingTolerance {
		tempTolerance = maxTxWaitingTolerance
	}
	t.txWaitingTolerance = tempTolerance
	t.calcOverdueTs = time.Now()
}

func (t *WatchedTxInfo) isOutdate() bool {
	return time.Now().After(t.genTs.Add(txTTL))
}

type txsFetcher struct {
	resultChan chan []*pb.Transaction
}

func makeTxsFetcher() txsFetcher {
	return txsFetcher{make(chan []*pb.Transaction, 1)}
}

// innerTxStore 存储待打包交易
type innerTxStore struct {
	sync.Mutex
	txs     map[string]*txWithTimeMs
	indexer map[string]string
}

func newInnerTxStore() *innerTxStore {
	return &innerTxStore{
		txs:     make(map[string]*txWithTimeMs),
		indexer: make(map[string]string),
	}
}
func (its *innerTxStore) addTx(tw *txWithTimeMs) {
	its.Lock()
	defer its.Unlock()
	txID := tw.tx.TxID
	key := txID.ToText()
	its.txs[key] = tw

	for _, ptx := range tw.tx.Vin {
		its.indexer[ptx.TxID] = key
	}
	for _, ptx := range tw.tx.Vout {
		its.indexer[ptx.TxID] = key
	}
}
func (its *innerTxStore) getTxByTxID(id *crypto.Digest256) *txWithTimeMs {
	its.Lock()
	defer its.Unlock()
	key := id.ToText()
	return its.txs[key]
}

func (its *innerTxStore) getByPubTxID(scTxID string) *txWithTimeMs {
	its.Lock()
	defer its.Unlock()
	index := its.indexer[scTxID]
	res := its.txs[index]
	return res
}
func (its *innerTxStore) getTxs() []*pb.Transaction {
	txs := make([]*pb.Transaction, 0)
	its.Lock()
	defer its.Unlock()
	for _, tx := range its.txs {
		txs = append(txs, tx.tx)
	}
	return txs
}

func (its *innerTxStore) delTx(tx *pb.Transaction) {
	its.Lock()
	defer its.Unlock()
	txID := tx.TxID
	key := txID.ToText()
	delete(its.txs, key)
	for _, ptx := range tx.Vin {
		delete(its.indexer, ptx.TxID)
	}
	for _, ptx := range tx.Vout {
		delete(its.indexer, ptx.TxID)
	}
}

//是否存在超时待打包交易
func (its *innerTxStore) hasTimeoutTx(db *dgwdb.LDBDatabase) bool {
	var innerTxHasOverdue bool
	its.Lock()
	defer its.Unlock()
	for k, txInfo := range its.txs {
		if txInfo.IsOverdue() {
			id, _ := crypto.TextToDigest256(k)
			if GetTxLookupEntry(db, id) != nil {
				delete(its.txs, k)
				continue
			}
			bsLogger.Error("pack tx timeout")
			innerTxHasOverdue = true
			txInfo.resetWaitingTolerance()
		}
	}
	return innerTxHasOverdue
}

// TxStore 公链监听到的交易以及网关本身交易的内存池
type TxStore struct {
	TxOverdueEvent *util.Event
	db             *dgwdb.LDBDatabase

	addWaitPackTxCh chan *pb.Transaction //添加等待打包ch
	waitPackingTx   *innerTxStore        //未打包进区块的交易

	pendingFetchers []txsFetcher
	currTerm        int64

	newTermChan   chan int64
	newCommitChan chan blockCommitted
	fetchTxsChan  chan txsFetcher
	addTxsChan    chan addTxsRequest

	watchedTxEvent     sync.Map //监听到的event
	addWatchedEventCh  chan *pb.WatchedEvent
	waitingSignTx      sync.Map //等待被签名交易
	addWaitingSignTxCh chan *message.WaitSignMsg
	//queryTxsChan   chan txsQuery
	heartbeatTimer *time.Timer
	sync.RWMutex
}

// NewTxStore 新建一个TxStore对象并返回
func NewTxStore(db *dgwdb.LDBDatabase) *TxStore {
	txStore := &TxStore{
		TxOverdueEvent:  util.NewEvent(),
		db:              db,
		addWaitPackTxCh: make(chan *pb.Transaction),
		pendingFetchers: nil,
		currTerm:        0,

		newTermChan:        make(chan int64),
		newCommitChan:      make(chan blockCommitted),
		addTxsChan:         make(chan addTxsRequest),
		fetchTxsChan:       make(chan txsFetcher),
		addWatchedEventCh:  make(chan *pb.WatchedEvent),
		addWaitingSignTxCh: make(chan *message.WaitSignMsg),
		//queryTxsChan:   make(chan txsQuery),
		heartbeatTimer: time.NewTimer(heartbeatInterval),
	}

	return txStore
}

// Run TxStore的循环处理函数
func (ts *TxStore) Run(ctx context.Context) {
	go ts.heartbeatCheck(ctx)
	for {
		select {
		case addTxsReq := <-ts.addTxsChan:
			addTxsReq.notAddedChan <- ts.addTxs(addTxsReq.txs)

		case newCommit := <-ts.newCommitChan:
			ts.cleanUpOnNewCommitted(newCommit.txs, newCommit.height)
		case term := <-ts.newTermChan:
			ts.currTerm = term
		case watchedEvent := <-ts.addWatchedEventCh: //添加监听到的event
			bsLogger.Debug("add watched event to mempool", "business", watchedEvent.GetBusiness(), "scTxID", watchedEvent.GetTxID())
			ts.watchedTxEvent.Store(watchedEvent.GetTxID(), watchedEvent)
		case waitSignTx := <-ts.addWaitingSignTxCh: //添加待签名交易
			bsLogger.Debug("add waitingSign tx", "business", waitSignTx.Business, "scTxID", waitSignTx.ScTxID)
			ts.waitingSignTx.Store(waitSignTx.ScTxID, newSignMsgWithtimeMs(waitSignTx))
		case <-ctx.Done():
			return
		}
	}
}

// HasWatchedEvent 是否已经监听到事件
func (ts *TxStore) HasWatchedEvent(tx *pb.WatchedEvent) bool {

	if tx != nil && tx.GetTxID() != "" {
		_, inWatched := ts.watchedTxEvent.Load(tx.GetTxID())
		inDB := ts.HasTxInDB(tx.GetTxID())
		return inWatched && inDB
	}
	return false
}

// OnNewBlockCommitted 新区块共识后的回调处理，需要清理内存池
func (ts *TxStore) OnNewBlockCommitted(newBlock *pb.BlockPack) {
	bsLogger.Debug("begin OnNewBlockCommitted", "blocktype", newBlock.Block().Type)
	if newBlock.IsTxsBlock() {
		req := makeBlockCommitted(newBlock)
		ts.newCommitChan <- req
	}
}

// OnTermChanged term更新时的处理
func (ts *TxStore) OnTermChanged(term int64) {
	// ts.newTermChan <- term
	bsLogger.Debug("set curr term", "term", term)
	ts.currTerm = term
}

// TestAddTxs just fot test api
func (ts *TxStore) TestAddTxs(txs []*pb.Transaction) []int {
	req := makeAddTxsRequest(txs)
	ts.addTxsChan <- req
	return <-req.notAddedChan
}

// AddWatchedEvent 添加监听到的事件到内存池
func (ts *TxStore) AddWatchedEvent(event *pb.WatchedEvent) {
	ts.addWatchedEventCh <- event
}

// AddTxtoWaitSign 重新加入待签名
func (ts *TxStore) AddTxtoWaitSign(msg *message.WaitSignMsg) {
	scTxID := msg.Event.GetTxID()
	inMem := ts.IsTxInMem(msg.ScTxID)
	if msg != nil && !ts.HasTxInDB(scTxID) && !inMem {
		go func() {
			ts.addWaitingSignTxCh <- msg
		}()
	} else {
		bsLogger.Debug("tx has been in db or mem", "scTxID", scTxID)
	}

}

// GetWaitingSignTxs 获取待签名交易
func (ts *TxStore) GetWaitingSignTxs() []*message.WaitSignMsg {

	var txs []*message.WaitSignMsg
	ts.waitingSignTx.Range(func(k, v interface{}) bool {
		tx, ok := v.(*message.WaitSignMsg)
		if ok {
			scTxID := tx.ScTxID
			// 本term内已经确定加签失败的交易，下个term再重新发起
			if IsSignFailed(ts.db, scTxID, ts.currTerm) {
				return true
			}
			if ts.HasTxInDB(scTxID) || ts.IsTxInMem(scTxID) {
				bsLogger.Debug("tx in fresh has been in mem or db", "scTxID", scTxID)
				ts.waitingSignTx.Delete(scTxID)
				return true
			}
			bsLogger.Debug("add watched tx to queue", "sctxid", scTxID)
			txs = append(txs, tx)
		}
		return true
	})
	ts.waitingSignTx = sync.Map{}
	return txs
}

// HasWaitSignTx 待签名交易是否为空
func (ts *TxStore) HasWaitSignTx() bool {
	var has bool
	ts.waitingSignTx.Range(func(k, v interface{}) bool {
		has = true
		return false
	})
	return has
}

// DeleteWaitSign 删除待签名交易
func (ts *TxStore) DeleteWaitSign(scTxID string) {
	ts.waitingSignTx.Delete(scTxID)
}

// DeleteWatchedEvent 删除监听到的事件
func (ts *TxStore) DeleteWatchedEvent(scTxID string) {
	ts.watchedTxEvent.Delete(scTxID)
}

// CreateInnerTx 创建一笔网关本身的交易
func (ts *TxStore) CreateInnerTx(innerTx *pb.Transaction) error {
	for _, putx := range innerTx.Vout {
		signMsg := GetSignReq(ts.db, putx.TxID)
		if signMsg == nil {
			bsLogger.Error("create inner tx failed, sign msg not found", "signmsgid", putx.TxID)
			return errors.New("vout has unsigned tx")
		}
	}
	txID := innerTx.TxID
	innerTx.UpdateId()
	if !bytes.Equal(txID.Data, innerTx.TxID.Data) {
		return errors.New("txid validate fail")
	}
	// 这个时候监听到的交易已经成功处理并上链了，先清理监听交易缓存
	for _, pubtx := range innerTx.Vin {
		ts.watchedTxEvent.Delete(pubtx.TxID)
		ts.waitingSignTx.Delete(pubtx.TxID)
	}
	for _, pubtx := range innerTx.Vout {
		ts.watchedTxEvent.Delete(pubtx.TxID)
		ts.waitingSignTx.Delete(pubtx.TxID)
	}
	innerTx.UpdateId()
	req := makeAddTxsRequest([]*pb.Transaction{innerTx})
	ts.addTxsChan <- req
	return nil
}

// QueryTxInfoBySidechainId 根据公链的交易id查询对应到的网关交易信息
func (ts *TxStore) QueryTxInfoBySidechainId(scId string) *TxQueryResult {
	return ts.queryTxsInfo([]string{scId})[0]
}

func (ts *TxStore) FetchTxsChan() <-chan []*pb.Transaction {
	fetcher := makeTxsFetcher()
	ts.fetchTxsChan <- fetcher
	return fetcher.resultChan
}

// GetMemTxs 获取内存池的网关交易
func (ts *TxStore) GetMemTxs() []*pb.Transaction {
	var fetched []*pb.Transaction
	fetched = ts.waitPackingTx.getTxs()
	return fetched
}

func (ts *TxStore) addTxs(txs []*pb.Transaction) []int {
	notAddedIds := make([]int, 0)
	ts.Lock()
	for idx, tx := range txs {
		if ts.waitPackingTx.getTxByTxID(tx.TxID) == nil && GetTxLookupEntry(ts.db, tx.TxID) == nil {
			tt := &txWithTimeMs{tx, time.Now(), time.Now(), defaultTxWaitingTolerance}
			ts.waitPackingTx.addTx(tt)
		} else {
			notAddedIds = append(notAddedIds, idx)
		}
	}
	ts.Unlock()
	return notAddedIds
}

// IsTxInMem 交易是否等待打包 代替 hasTxInMemPool
func (ts *TxStore) IsTxInMem(scTxID string) bool {
	tx := ts.waitPackingTx.getByPubTxID(scTxID)
	return tx != nil
}

func (ts *TxStore) HasTxInDB(scTxId string) bool {
	tmp := GetTxIdBySidechainTxId(ts.db, scTxId)
	return tmp != nil
}

// 把被打包进新区块的交易从内存池里面删掉
func (ts *TxStore) cleanUpOnNewCommitted(committedTxs []*pb.Transaction, height int64) {
	for idx, tx := range committedTxs {
		entry := &pb.TxLookupEntry{
			Height: height,
			Index:  int32(idx),
		}

		SetTxLookupEntry(ts.db, tx.TxID, entry)
		//from链 tx_id 和网关tx_id的对应
		for _, pubTx := range tx.Vin {
			bsLogger.Debug("write tx to db and delete from mempool", "txid", pubTx.GetTxID())
			SetTxIdMap(ts.db, pubTx.GetTxID(), tx.TxID)
			ts.watchedTxEvent.Delete(pubTx.GetTxID())
			ts.waitingSignTx.Delete(pubTx.GetTxID())
		}
		//to链和tx_id 和网关tx_id的对应
		for _, pubTx := range tx.Vout {
			bsLogger.Debug("write tx to db and delete from mempool", "txid", pubTx.GetTxID())
			SetTxIdMap(ts.db, pubTx.GetTxID(), tx.TxID)
			ts.watchedTxEvent.Delete(pubTx.GetTxID())
			ts.waitingSignTx.Delete(pubTx.GetTxID())
		}
		ts.waitPackingTx.delTx(tx)
	}
}

func (ts *TxStore) queryTxsInfo(scTxIds []string) (rst []*TxQueryResult) {
	for _, scTxId := range scTxIds {
		value := ts.waitPackingTx.getByPubTxID(scTxId)
		if value != nil {
			rst = append(rst, &TxQueryResult{
				Tx:     value.tx,
				Height: -1,
			})
		} else {
			txId := GetTxIdBySidechainTxId(ts.db, scTxId)
			if txId == nil {
				bsLogger.Debug("get tx id failed", "sideId", scTxId)
				rst = append(rst, nil)
				continue
			}
			entry := GetTxLookupEntry(ts.db, txId)
			if entry == nil {
				bsLogger.Debug("query transaction not found", "txId", txId)
				rst = append(rst, nil)
				continue
			}
			assert.True(entry.Height <= GetCommitHeight(ts.db))
			blockPack := GetCommitByHeight(ts.db, entry.Height)
			if blockPack == nil {
				bsLogger.Debug("query transaction block not found", "height", entry.Height)
				rst = append(rst, nil)
				continue
			}
			assert.True(int(entry.Index) < len(blockPack.Block().Txs) && entry.Index >= 0)
			tx := blockPack.Block().Txs[entry.Index]
			rst = append(rst, &TxQueryResult{
				Tx:      tx,
				Height:  entry.Height,
				BlockID: blockPack.Id(),
			})
		}
	}
	return
}

func (ts *TxStore) GetTx(txid string) *TxQueryResult {
	dgwTxID := GetTxIdBySidechainTxId(ts.db, txid) //首先通过公链tx_id查询网关tx_id
	if dgwTxID == nil {                            //如果获取不到使用网关tx_id查询
		bsLogger.Debug("get dgw from txid failed", "sideId", txid)
		data, err := hex.DecodeString(txid)
		if err != nil {
			return nil
		}
		dgwTxID = &crypto.Digest256{
			Data: data,
		}
	}
	entry := GetTxLookupEntry(ts.db, dgwTxID)
	if entry == nil {
		bsLogger.Debug("query transaction not found", "txId", txid)
		return nil
	}
	assert.True(entry.Height <= GetCommitHeight(ts.db))
	blockPack := GetCommitByHeight(ts.db, entry.Height)
	if blockPack == nil {
		bsLogger.Debug("query transaction block not found", "height", entry.Height)
		return nil
	}
	assert.True(int(entry.Index) < len(blockPack.Block().Txs) && entry.Index >= 0)
	tx := blockPack.Block().Txs[entry.Index]
	return &TxQueryResult{
		Tx:      tx,
		Height:  entry.Height,
		BlockID: blockPack.Id(),
	}
}

func (ts *TxStore) heartbeatCheck(ctx context.Context) {
	for {
		select {
		case <-ts.heartbeatTimer.C:
			ts.doHeartbeat()
			ts.heartbeatTimer.Reset(heartbeatInterval)
		case <-ctx.Done():
			return
		}
	}
}

func (ts *TxStore) doHeartbeat() {
	innerTxHasOverdue, watchedTxHasOverdue := false, false

	ts.waitingSignTx.Range(func(k, v interface{}) bool {
		msg := v.(*signMsgWithtimeMs)
		if msg.IsOverdue() {
			scTxID := msg.msg.ScTxID
			if ts.waitPackingTx.getByPubTxID(scTxID) != nil {
				return true
			}
			if GetTxIdBySidechainTxId(ts.db, scTxID) != nil {
				ts.watchedTxEvent.Delete(scTxID) //删除监听到的event
				ts.waitingSignTx.Delete(scTxID)  //删除等待签名的请求
				return true
			}
			bsLogger.Error("wait sign tx timeout", "sctxid", scTxID)
			watchedTxHasOverdue = true
			// 重新设置超时时间 防止重复accuse
			msg.resetWaitingTolerance()
		}
		return true
	})

	innerTxHasOverdue = ts.waitPackingTx.hasTimeoutTx(ts.db)

	if innerTxHasOverdue || watchedTxHasOverdue {
		ts.TxOverdueEvent.Emit(GetNodeTerm(ts.db))
	}
}

func (ts *TxStore) validatePubTxs(txs []*pb.PublicTx) int {
	for _, pubTx := range txs {
		val, ok := ts.watchedTxEvent.Load(pubTx.TxID)
		if !ok {
			// 如果本节点没有监听记录，则返回mempool里面不存在
			// 很可能是新加入的节点，跳过本次区块，下一次init区块的时候会去自动同步最新区块
			bsLogger.Warn("validate tx not exist", "sctxid", pubTx.TxID)
			return NotExist
		}
		watchedEvent := val.(*pb.WatchedEvent)
		if watchedEvent.GetFrom() != pubTx.Chain && !bytes.Equal(watchedEvent.Data, pubTx.Data) {
			bsLogger.Warn("validate watchedtx invalid", "scTxID", pubTx.TxID)
			return Invalid
		}
	}
	return CheckChain
}

// ValidateTx 验证交易的合法性
func (ts *TxStore) ValidateTx(tx *pb.Transaction) int {
	memTx := ts.waitPackingTx.getTxByTxID(tx.TxID)
	if memTx == nil {
		validateRes := ts.validatePubTxs(tx.Vin)
		if validateRes == NotExist || validateRes == Invalid {
			bsLogger.Warn("validate vin pub tx fail")
			return validateRes
		}
		// 与本机watched的消息一致，但本机还没有监听到公链上的交易结果
		// 返回CheckChain直接去公链上验证
		return CheckChain
	}
	if memTx.tx.EqualTo(tx) {
		return Valid
	}
	// 证明本机监听到的消息与传过来的消息不一致，先accuse再说
	bsLogger.Warn("validate gateway tx invalid", "innertxID", tx.TxID)
	return Invalid

}

// ValidateTx 验证交易的合法性
/*
func (ts *TxStore) ValidateTx(tx *pb.Transaction) int {
	ts.RLock()
	defer ts.RUnlock()
	memTx := ts.txInfoById[tx.WatchedTx.Txid]
	if memTx == nil {
		watchedTx := ts.watchedTxInfo[tx.WatchedTx.Txid]
		if watchedTx == nil {
			// 如果本节点没有监听记录，则返回mempool里面不存在
			// 很可能是新加入的节点，跳过本次区块，下一次init区块的时候会去自动同步最新区块
			bsLogger.Warn("validate tx not exist", "sctxid", tx.WatchedTx.Txid)
			return NotExist
		} else if !watchedTx.Tx.EqualTo(tx.WatchedTx) {
			bsLogger.Warn("validate watchedtx invalid", "tx", tx.WatchedTx, "memtx", watchedTx)
			return Invalid
		} else {
			// 与本机watched的消息一致，但本机还没有监听到公链上的交易结果
			// 返回CheckChain直接去公链上验证
			return CheckChain
		}
	} else {
		if memTx.tx.EqualTo(tx) {
			return Valid
		}
		// 证明本机监听到的消息与传过来的消息不一致，先accuse再说
		bsLogger.Warn("validate gateway tx invalid", "sctxid", tx.WatchedTx.Txid)
		return Invalid
	}
}
*/

// ValidateWatchedEvent 验证leader传过来的PushEvent是否与本节点一致
func (ts *TxStore) ValidateWatchedEvent(event *pb.WatchedEvent) int {
	val, exist := ts.watchedTxEvent.Load(event.GetTxID())
	if !exist {
		return NotExist
	}
	watchedEvent := val.(*pb.WatchedEvent)
	if watchedEvent.Equal(event) {
		return Valid
	}
	bsLogger.Error("watched event validate fail", "business", event.GetBusiness(), "scTxID", event.GetTxID())
	return Invalid
}
