package node

import (
	"bytes"
	"sync"
	"time"

	btwatcher "swap/btwatcher"

	"github.com/ofgp/common/defines"
	"github.com/ofgp/ofgp-core/cluster"
	pb "github.com/ofgp/ofgp-core/proto"
	"github.com/ofgp/ofgp-core/util/assert"

	"sync/atomic"

	"github.com/btcsuite/btcd/wire"
	context "golang.org/x/net/context"
)

var signTimeout int64 //单位s

const defaultSignSucTimeout = 15    //单位s
var checkSignInterval time.Duration //checkSign是否达成共识的周期

//统计sign suc的节点数
type SignedResultCache struct {
	cache       map[int32]*pb.SignResult
	totalCount  int32
	signedCount int32
	errCnt      int32
	doneTime    time.Time
	initTime    int64
	sync.RWMutex
	doneFlag int32
}

func (cache *SignedResultCache) setDone() bool {
	return atomic.CompareAndSwapInt32(&cache.doneFlag, 0, 1)
}
func (cache *SignedResultCache) isDone() bool {
	return atomic.LoadInt32(&cache.doneFlag) == 1
}

func (cache *SignedResultCache) addTotalCount() {
	atomic.AddInt32(&cache.totalCount, 1)
}
func (cache *SignedResultCache) addSignedCount() {
	atomic.AddInt32(&cache.signedCount, 1)
}
func (cache *SignedResultCache) addErrCnt() {
	atomic.AddInt32(&cache.errCnt, 1)
}
func (cache *SignedResultCache) addCache(nodeID int32, signRes *pb.SignResult) {
	cache.Lock()
	defer cache.Unlock()
	cache.cache[nodeID] = signRes
}
func (cache *SignedResultCache) getCache(nodeID int32) (*pb.SignResult, bool) {
	cache.RLock()
	defer cache.RUnlock()
	res, ok := cache.cache[nodeID]
	return res, ok
}

// getTotalCount 获取收到签名结果的个数
func (cache *SignedResultCache) getTotalCount() int32 {
	return atomic.LoadInt32(&cache.totalCount)
}

// getSignedCount 获取收到签名成功的个数
func (cache *SignedResultCache) getSignedCount() int32 {
	return atomic.LoadInt32(&cache.signedCount)
}

func (cache *SignedResultCache) getErrCnt() int32 {
	return atomic.LoadInt32(&cache.errCnt)
}

func (node *BraftNode) clearOnFail(signReq *pb.SignRequest) {
	scTxID := signReq.GetWatchedEvent().GetTxID()
	leaderLogger.Debug("clear on fail", "sctxid", scTxID)
	node.blockStore.MarkFailedSignRecord(scTxID, signReq.Term)

	node.signedResultCache.Delete(scTxID)
	node.blockStore.DeleteSignReqMsg(scTxID)

}

/*
func (node *BraftNode) sendTxToChain(newlyTx *wire.MsgTx, watcher *btcwatcher.MortgageWatcher,
	sigs [][][]byte, signResult *pb.SignedResult, signReq *pb.SignTxRequest) {

	// mergesigs的顺序需要与生成多签地址的顺序严格一致，所以按nodeid来顺序添加返回的sig
	newlyTxHash := newlyTx.TxHash().String()
	ok := watcher.MergeSignTx(newlyTx, sigs)
	if !ok {
		leaderLogger.Error("merge sign tx failed", "sctxid", signResult.TxId)
		// node.clearOnFail(signReq)  merge 失败clearOnFail 处理
		return
	}
	start := time.Now().UnixNano()
	_, err := watcher.SendTx(newlyTx)
	end := time.Now().UnixNano()
	leaderLogger.Debug("sendBchtime", "time", (end-start)/1e6)
	if err != nil {
		leaderLogger.Error("send signed tx to bch failed", "err", err, "sctxid", signResult.TxId)
	}
	node.blockStore.SignedTxEvent.Emit(newlyTxHash, signResult.TxId, signResult.To, signReq.WatchedTx.TokenTo)
}
*/

func createMortgageTx(req *pb.SignRequest) interface{} {
	buf := bytes.NewBuffer(req.NewlyTx.Data)
	newlyTx := new(wire.MsgTx)
	err := newlyTx.Deserialize(buf)
	assert.ErrorIsNil(err)
	return newlyTx
}
func checkMortgageTx(watcher *btwatcher.Watcher, tx interface{}, res *pb.SignResult) bool {
	if newlyTx, ok := tx.(*wire.MsgTx); ok {
		checkRes := watcher.VerifySign(newlyTx, res.Data, cluster.NodeList[res.NodeID].PublicKey)
		return checkRes
	}
	leaderLogger.Error("btc tx type is wrong")
	return false
}
func mergeMortgageTx(watcher *btwatcher.Watcher, tx interface{},
	results []*pb.SignResult, quorumN int) (interface{}, string, bool) {
	var btcTx *wire.MsgTx
	var ok bool
	if btcTx, ok = tx.(*wire.MsgTx); !ok {
		return nil, "", false
	}
	beforeSignTxID := btcTx.TxHash().String()
	var sigs [][][]byte
	for _, res := range results {
		sigs = append(sigs, res.Data)
		if len(sigs) == int(quorumN) {
			break
		}
	}
	ok = watcher.MergeSignTx(btcTx, sigs)
	return tx, beforeSignTxID, ok
}

type collector interface {
	createTx(req *pb.SignRequest) interface{}
	check(tx interface{}, res *pb.SignResult) bool
	merge(tx interface{}, results []*pb.SignResult, quorumN int) (interface{}, string, bool)
}

// btcResCollector btc收集签名结果
type btcResCollector struct {
	watcher *btwatcher.Watcher
}

func newBtcResCollector(watcher *btwatcher.Watcher) *btcResCollector {
	return &btcResCollector{
		watcher: watcher,
	}
}
func (brc *btcResCollector) createTx(req *pb.SignRequest) interface{} {
	return createMortgageTx(req)
}
func (brc *btcResCollector) check(tx interface{}, res *pb.SignResult) bool {
	return checkMortgageTx(brc.watcher, tx, res)
}

func (brc *btcResCollector) merge(tx interface{}, results []*pb.SignResult, quorumN int) (interface{}, string, bool) {
	return mergeMortgageTx(brc.watcher, tx, results, quorumN)
}

//bchResCollector bch收集签名结果
type bchResCollector struct {
	watcher *btwatcher.Watcher
}

func newBchResCollector(watcher *btwatcher.Watcher) *bchResCollector {
	return &bchResCollector{
		watcher: watcher,
	}
}

func (brc *bchResCollector) createTx(req *pb.SignRequest) interface{} {
	return createMortgageTx(req)
}
func (brc *bchResCollector) check(tx interface{}, res *pb.SignResult) bool {
	return checkMortgageTx(brc.watcher, tx, res)
}

func (brc *bchResCollector) merge(tx interface{}, results []*pb.SignResult, quorumN int) (interface{}, string, bool) {
	return mergeMortgageTx(brc.watcher, tx, results, quorumN)
}

type ethResCollector struct {
}

func newEthResCollector() *ethResCollector {
	return &ethResCollector{}
}

func (erc *ethResCollector) createTx(req *pb.SignRequest) interface{} {
	return nil
}
func (erc *ethResCollector) check(tx interface{}, res *pb.SignResult) bool {
	return true
}
func (erc *ethResCollector) merge(tx interface{}, results []*pb.SignResult, quorumN int) (interface{}, string, bool) {
	return nil, "", true
}

type collectorFactory struct {
	bch *bchResCollector
	btc *btcResCollector
	eth *ethResCollector
}

func newCollectorFactory(bch *bchResCollector, btc *btcResCollector, eth *ethResCollector) *collectorFactory {
	return &collectorFactory{
		bch: bch,
		btc: btc,
		eth: eth,
	}
}

func (factory *collectorFactory) getCollector(chain uint32) collector {
	chainCmp := uint8(chain)
	switch chainCmp {
	case defines.CHAIN_CODE_BCH:
		return factory.bch
	case defines.CHAIN_CODE_BTC:
		return factory.btc
	case defines.CHAIN_CODE_ETH:
		return factory.eth
	default:
		leaderLogger.Error("can't get collector")
		return nil
	}
}

func (node *BraftNode) doSave(msg *pb.SignResult) {
	// var watcher *btcwatcher.MortgageWatcher
	collector := node.collectorFactory.getCollector(msg.GetTo())
	scTxID := msg.GetScTxID()
	if node.blockStore.IsSignFailed(scTxID, msg.Term) {
		leaderLogger.Debug("signmsg is failed in this term", "sctxid", scTxID, "term", msg.Term)
		return
	}
	signReq := node.blockStore.GetSignReq(scTxID)
	cacheTemp, loaded := node.signedResultCache.LoadOrStore(scTxID, &SignedResultCache{
		cache:       make(map[int32]*pb.SignResult),
		totalCount:  0,
		signedCount: 0,
		initTime:    time.Now().Unix(),
	})

	cache, ok := cacheTemp.(*SignedResultCache)
	if !ok {
		return
	}
	//如果不是第一次添加
	if loaded {
		if _, exist := cache.getCache(msg.NodeID); exist {
			leaderLogger.Debug("already receive signedres", "nodeID", msg.NodeID, "scTxID", scTxID)
			return
		}
	}
	cache.addTotalCount()
	// 由于网络延迟，有可能先收到了其他节点的签名结果，后收到签名请求，这个时候只做好保存即可
	if signReq == nil {
		leaderLogger.Debug("signReq is nil", "scTxID", scTxID)
		if msg.Code == pb.CodeType_SIGNED {
			cache.addSignedCount()
			cache.addCache(msg.NodeID, msg)
		} else {
			cache.addErrCnt()
		}
		return
	}
	newlyTx := collector.createTx(signReq)

	if msg.Code == pb.CodeType_SIGNED {
		// if !watcher.VerifySign(newlyTx, msg.Data, cluster.NodeList[msg.NodeID].PublicKey) {
		// 	leaderLogger.Error("verify sign tx failed", "from", msg.NodeID, "sctxid", scTxID, "business", msg.Business)
		// 	cache.addErrCnt()
		// }
		if !collector.check(newlyTx, msg) {
			leaderLogger.Error("verify sign tx failed", "from", msg.NodeID, "sctxid", scTxID, "business", msg.Business)
			cache.addErrCnt()
		} else {
			cache.addCache(msg.NodeID, msg)
			cache.addSignedCount()
		}
	} else {
		cache.addErrCnt()
	}

	var (
		quorumN       int32
		accuseQuorumN int32
	)

	if signReq.GetWatchedEvent().IsTransferEvent() {
		snapshot := node.blockStore.GetClusterSnapshot(signReq.MultisigAddress)
		if snapshot == nil {
			leaderLogger.Error("receive invalid transfer tx", "txid", scTxID)
			return
		}
		quorumN = int32(snapshot.QuorumN)
		accuseQuorumN = int32(snapshot.AccuseQuorumN)
	} else {
		quorumN = int32(cluster.QuorumN)
		accuseQuorumN = int32(cluster.AccuseQuorumN)
	}

	if cache.getSignedCount() >= quorumN && !cache.isDone() && signReq != nil {
		if cache.setDone() {
			cache.doneTime = time.Now()
			// var sigs [][][]byte
			// for idx := range cluster.NodeList {
			// 	if result, has := cache.getCache(int32(idx)); has {
			// 		leaderLogger.Debug("will merge sign info", "from", idx)
			// 		sigs = append(sigs, result.Data)
			// 		if len(sigs) == int(quorumN) {
			// 			break
			// 		}
			// 	}
			// }
			// 获取cache的签名结果
			signResults := make([]*pb.SignResult, 0)
			for idx := range cluster.NodeList {
				if result, has := cache.getCache(int32(idx)); has {
					signResults = append(signResults, result)
					if len(signResults) == int(quorumN) {
						break
					}
				}
			}
			// ok := watcher.MergeSignTx(newlyTx, sigs)
			var signBeforeTxID string
			newlyTx, signBeforeTxID, ok = collector.merge(newlyTx, signResults, int(quorumN))
			if !ok {
				leaderLogger.Error("merge sign tx failed", "business", msg.Business, "sctxID", msg.ScTxID)
				node.clearOnFail(signReq) // merge 失败clearOnFail 处理
				return
			}
			//标记已签名 替换SignedEvent
			node.markTxSigned(msg.ScTxID)
			//通知相关业务已被签名
			node.pubSigned(msg, msg.To, newlyTx, signBeforeTxID, signReq.Term)
			// sendTxToChain的时间可能会比较长，因为涉及到链上交易，所以需要提前把锁释放
			// node.sendTxToChain(newlyTx, watcher, sigs, msg, signReqmsg)
		}
	} else if cache.getErrCnt() > accuseQuorumN {
		// 本次交易确认失败，清理缓存的数据，避免干扰后续的重试
		leaderLogger.Debug("sign accuseQuorumN fail", "business", msg.Business, "scTxID", msg.ScTxID)
		node.clearOnFail(signReq)
	}
}

func (node *BraftNode) saveSignedResult(ctx context.Context) {
	//定期删除已处理的sign
	clearCh := time.NewTicker(cluster.BlockInterval).C
	for {
		select {
		case msg := <-node.signedResultChan:
			go node.doSave(msg)
		case <-clearCh:
			node.signedResultCache.Range(func(k, v interface{}) bool {
				txID := k.(string)
				cache := v.(*SignedResultCache)
				if cache.isDone() && time.Now().After(cache.doneTime.Add(cacheTimeout)) {
					node.signedResultCache.Delete(txID)
				}
				return true
			})
		case <-ctx.Done():
			return
		}
	}
}

// isTxSigned 判断是否已被签名
func (node *BraftNode) isTxSigned(scTxID string) bool {
	_, ok := node.signedTxs.Load(scTxID)
	return ok
}

func (node *BraftNode) checkSignTimeout() {
	node.signedResultCache.Range(func(k, v interface{}) bool {
		scTxID := k.(string)
		cache := v.(*SignedResultCache)
		now := time.Now().Unix()
		if now-cache.initTime > signTimeout && !cache.isDone() { //sign达成共识超时，重新放回处理队列

			leaderLogger.Debug("sign timeout", "scTxID", scTxID)

			//删除sign标记
			node.signedResultCache.Delete(scTxID)
			signReq := node.blockStore.GetSignReq(scTxID)
			if signReq == nil { //本地尚未签名
				return true
			}
			node.blockStore.DeleteSignReqMsg(scTxID)
			watchedEvent := signReq.GetWatchedEvent()
			if watchedEvent.IsTransferEvent() {
				node.txStore.DeleteWatchedEvent(scTxID)
			}
			// if !watchedEvent.IsTransferEvent() {
			// if !node.isTxSigned(scTxID) { //如果签名已经共识
			// 	node.txStore.AddTxtoWaitSign(&message.WaitSignMsg{
			// 		Business: signReq.Business,
			// 		ScTxID:   signReq.GetWatchedEvent().GetTxID(),
			// 		Event:    signReq.WatchedEvent,
			// 		Tx:       signReq.NewlyTx,
			// 	})
			// } else {
			// 	nodeLogger.Debug("tx is in waiting", "scTxID", scTxID)
			// }
			// } else {
			// 	node.txStore.DeleteWatchedEvent(scTxID)
			// }
		}
		return true

	})
}

func (node *BraftNode) runCheckSignTimeout(ctx context.Context) {
	go func() {
		if checkSignInterval == 0 {
			checkSignInterval = cluster.BlockInterval
		}
		if signTimeout == 0 {
			signTimeout = defaultSignSucTimeout
		}
		tch := time.NewTicker(checkSignInterval).C
		for {
			select {
			case <-tch:
				node.checkSignTimeout()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (node *BraftNode) SaveSignedResult(msg *pb.SignResult) {
	node.signedResultChan <- msg
}
