package node

import (
	"bytes"
	"sync"
	"time"

	btcwatcher "github.com/ofgp/bitcoinWatcher/mortgagewatcher"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/message"
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

	//todo waitingConfirm 如何处理
	// if !signReq.GetWatchedEvent().IsTransferEvent() && !node.hasTxInWaitting(scTxID) {
	// 	node.txStore.AddFreshWatchedTx(signReq.WatchedTx)
	// } else {
	// 	node.txStore.DeleteWatchedTx(signReq.WatchedTx.Txid)
	// }
}

func (node *BraftNode) sendTxToChain(newlyTx *wire.MsgTx, watcher *btcwatcher.MortgageWatcher,
	sigs [][][]byte, signResult *pb.SignedResult, signReq *pb.SignTxRequest) {

	// mergesigs的顺序需要与生成多签地址的顺序严格一致，所以按nodeid来顺序添加返回的sig
	newlyTxHash := newlyTx.TxHash().String()
	ok := watcher.MergeSignTx(newlyTx, sigs)
	if !ok {
		leaderLogger.Error("merge sign tx failed", "sctxid", signResult.TxId)
		// node.clearOnFail(signReq) todo merge 失败clearOnFail 处理
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

func (node *BraftNode) doSave(msg *pb.SignResult) {
	var watcher *btcwatcher.MortgageWatcher
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
		if msg.Code == pb.CodeType_SIGNED {
			cache.addSignedCount()
			cache.addCache(msg.NodeID, msg)
		} else {
			cache.addErrCnt()
		}
		return
	}
	buf := bytes.NewBuffer(signReq.NewlyTx.Data)
	newlyTx := new(wire.MsgTx)
	err := newlyTx.Deserialize(buf)
	assert.ErrorIsNil(err)
	if msg.To == message.Bch {
		watcher = node.bchWatcher
	} else {
		watcher = node.btcWatcher
	}
	if msg.Code == pb.CodeType_SIGNED {
		if !watcher.VerifySign(newlyTx, msg.Data, cluster.NodeList[msg.NodeID].PublicKey) {
			leaderLogger.Error("verify sign tx failed", "from", msg.NodeID, "sctxid", scTxID)
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
			var sigs [][][]byte
			for idx := range cluster.NodeList {
				if result, has := cache.getCache(int32(idx)); has {
					leaderLogger.Debug("will merge sign info", "from", idx)
					sigs = append(sigs, result.Data)
					if len(sigs) == int(quorumN) {
						break
					}
				}
			}
			// sendTxToChain的时间可能会比较长，因为涉及到链上交易，所以需要提前把锁释放
			// node.sendTxToChain(newlyTx, watcher, sigs, msg, signReqmsg)
		}
	} else if cache.getErrCnt() > accuseQuorumN {
		// 本次交易确认失败，清理缓存的数据，避免干扰后续的重试
		leaderLogger.Debug("sign accuseQuorumN fail")
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

func (node *BraftNode) hasTxInWaitting(scTxID string) bool {
	node.mu.Lock()
	defer node.mu.Unlock()
	_, ok := node.waitingConfirmTxs[scTxID]
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
			signReq := node.blockStore.GetSignReqMsg(scTxID)
			if signReq == nil { //本地尚未签名
				return true
			}
			node.blockStore.DeleteSignReqMsg(scTxID)

			if !signReq.WatchedTx.IsTransferTx() {
				if !node.hasTxInWaitting(scTxID) { //如果签名已经共识
					node.txStore.AddFreshWatchedTx(signReq.WatchedTx)
				} else {
					nodeLogger.Debug("tx is in waiting", "scTxID", scTxID)
				}
			} else {
				node.txStore.DeleteWatchedTx(scTxID)
			}
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
