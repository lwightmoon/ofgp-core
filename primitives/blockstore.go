package primitives

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/dgwdb"
	"github.com/ofgp/ofgp-core/log"
	"github.com/ofgp/ofgp-core/message"
	pb "github.com/ofgp/ofgp-core/proto"
	"github.com/ofgp/ofgp-core/util"
	"github.com/ofgp/ofgp-core/util/assert"
	"github.com/ofgp/ofgp-core/util/task"

	btcfunc "github.com/ofgp/bitcoinWatcher/coinmanager"
	btcwatcher "github.com/ofgp/bitcoinWatcher/mortgagewatcher"

	ew "github.com/ofgp/ethwatcher"

	"github.com/btcsuite/btcd/wire"
	"github.com/spf13/viper"
)

var (
	bsLogger = log.New(viper.GetString("loglevel"), "blockstore")
	mu       sync.RWMutex
)

const (
	validateSignTxResult = iota
	validatePass
	wrongInputOutput
	alreadySigned
)

// BlockStore 负责区块的处理，整个共识机制
type BlockStore struct {
	db           *dgwdb.LDBDatabase
	ts           *TxStore
	signer       *crypto.SecureSigner
	bchWatcher   *btcwatcher.MortgageWatcher
	btcWatcher   *btcwatcher.MortgageWatcher
	ethWatcher   *ew.Client
	signedTxMap  map[string]string
	localNodeId  int32
	prepareCache map[int64]map[int64][]*pb.PrepareMsg
	commitCache  map[int64]map[int64][]*pb.CommitMsg

	NeedSyncUpEvent            *util.Event
	NewInitedEvent             *util.Event
	NewPreparedEvent           *util.Event
	NewCommittedEvent          *util.Event
	CommittedInLowerTermEvent  *util.Event
	NewTermEvent               *util.Event
	NewWeakAccuseEvent         *util.Event
	NewStrongAccuseEvent       *util.Event
	StrongAccuseProcessedEvent *util.Event
	BroadcastSignResEvent      *util.Event //广播签名结果
	OnJoinEvent                *util.Event
	JoinedEvent                *util.Event
	JoinCancelEvent            *util.Event
	OnLeaveEvent               *util.Event
	LeavedEvent                *util.Event
	LeaveCancelEvent           *util.Event
	ReconfigEvent              *util.Event
}

// NewBlockStore 生成一个BlockStore对象
func NewBlockStore(db *dgwdb.LDBDatabase, ts *TxStore, btcWatcher *btcwatcher.MortgageWatcher, bchWatcher *btcwatcher.MortgageWatcher,
	ethWatcher *ew.Client, signer *crypto.SecureSigner, localNodeId int32) *BlockStore {
	return &BlockStore{
		db:           db,
		ts:           ts,
		signer:       signer,
		bchWatcher:   bchWatcher,
		btcWatcher:   btcWatcher,
		ethWatcher:   ethWatcher,
		signedTxMap:  make(map[string]string),
		localNodeId:  localNodeId,
		prepareCache: make(map[int64]map[int64][]*pb.PrepareMsg),
		commitCache:  make(map[int64]map[int64][]*pb.CommitMsg),

		NeedSyncUpEvent:            util.NewEvent(),
		NewInitedEvent:             util.NewEvent(),
		NewPreparedEvent:           util.NewEvent(),
		NewCommittedEvent:          util.NewEvent(),
		CommittedInLowerTermEvent:  util.NewEvent(),
		NewTermEvent:               util.NewEvent(),
		NewWeakAccuseEvent:         util.NewEvent(),
		NewStrongAccuseEvent:       util.NewEvent(),
		StrongAccuseProcessedEvent: util.NewEvent(),
		BroadcastSignResEvent:      util.NewEvent(),
		OnJoinEvent:                util.NewEvent(),
		JoinedEvent:                util.NewEvent(),
		JoinCancelEvent:            util.NewEvent(),
		OnLeaveEvent:               util.NewEvent(),
		LeavedEvent:                util.NewEvent(),
		LeaveCancelEvent:           util.NewEvent(),
		ReconfigEvent:              util.NewEvent(),
	}
}

// GetNodeTerm 获取节点的term
func (bs *BlockStore) GetNodeTerm() int64 {
	mu.RLock()
	defer mu.RUnlock()
	return GetNodeTerm(bs.db)
}

// GetFresh 获取节点当前共识中的block
func (bs *BlockStore) GetFresh() *pb.BlockPack {
	mu.RLock()
	defer mu.RUnlock()
	return GetFresh(bs.db)
}

// GetCommitTop 获取当前的最新区块
func (bs *BlockStore) GetCommitTop() *pb.BlockPack {
	mu.RLock()
	defer mu.RUnlock()
	return GetCommitTop(bs.db)
}

// GetCommitHeight 获取当前区块高度
func (bs *BlockStore) GetCommitHeight() int64 {
	return GetCommitHeight(bs.db)
}

// GetCommitByHeight 获取指定高度的区块
func (bs *BlockStore) GetCommitByHeight(height int64) *pb.BlockPack {
	return GetCommitByHeight(bs.db, height)
}

// GetBlockByID 根据blockhash 获取区块
func (bs *BlockStore) GetBlockByID(id []byte) *pb.BlockPack {
	return GetBlockByID(bs.db, id)
}

// GetCommitsByHeightSec 根据height 区间获取区块
func (bs *BlockStore) GetCommitsByHeightSec(start, end int64) []*pb.BlockPack {
	return GetCommitsByHeightSec(bs.db, start, end)
}

// IsCommitted 判断指定区块是否已经commited
func (bs *BlockStore) IsCommitted(blockId *crypto.Digest256) bool {
	mu.RLock()
	defer mu.RUnlock()
	return IsCommitted(bs.db, blockId)
}

// GetVotie 获取缓存的投票信息
func (bs *BlockStore) GetVotie() *pb.Votie {
	mu.RLock()
	defer mu.RUnlock()
	return GetVotie(bs.db)
}

// SetCurrentHeight 设置当前监听到的公链高度
func (bs *BlockStore) SetCurrentHeight(chainType string, height int64) {
	mu.Lock()
	SetCurrentHeight(bs.db, chainType, height)
	mu.Unlock()
}

// GetCurrentHeight 获取当前监听到的指定公链的高度
func (bs *BlockStore) GetCurrentHeight(chainType string) int64 {
	mu.RLock()
	defer mu.RUnlock()
	height := GetCurrentHeight(bs.db, chainType)
	return height
}

// DeleteSignReqMsg 删除缓存的签名请求
func (bs *BlockStore) DeleteSignReqMsg(txId string) {
	DeleteSignMsg(bs.db, txId)
}

// GetSignReqMsg 获取缓存的签名源请求
func (bs *BlockStore) GetSignReq(txId string) *pb.SignRequest {
	return GetSignReq(bs.db, txId)
}

// DeleteSignReqMsg 删除缓存的签名请求
func (bs *BlockStore) DeleteSignReq(txId string) {
	DeleteSignMsg(bs.db, txId)
}

// MarkFailedSignRecord 标记此term下的签名是否已经确认失败，需要重签
func (bs *BlockStore) MarkFailedSignRecord(txId string, term int64) {
	MarkFailedSignRecord(bs.db, txId, term)
}

// IsSignFailed 判断此term下的签名是否已经确认失败
func (bs *BlockStore) IsSignFailed(txId string, term int64) bool {
	return IsSignFailed(bs.db, txId, term)
}

// AddTransaction 测试接口
func (bs *BlockStore) AddTransaction(tx *pb.Transaction) {
	bs.ts.TestAddTxs([]*pb.Transaction{tx})
}

// DeleteJoinNodeInfo 删除缓存的JoinRequest
func (bs *BlockStore) DeleteJoinNodeInfo() {
	DeleteJoinNodeInfo(bs.db)
}

// DeleteLeaveNodeInfo 删除缓存的LeaveMessage
func (bs *BlockStore) DeleteLeaveNodeInfo() {
	DeleteLeaveNodeInfo(bs.db)
}

// SetETHBlockHeight 保存ETH当前监听到的高度
func (bs *BlockStore) SetETHBlockHeight(height *big.Int) {
	SetETHBlockHeight(bs.db, height)
}

// GetETHBlockHeight 获取上次ETH监听到的高度
func (bs *BlockStore) GetETHBlockHeight() *big.Int {
	return GetETHBlockHeight(bs.db)
}

// SetETHBlockTxIndex 保存当前ETH监听到的区块里面的哪一笔交易
func (bs *BlockStore) SetETHBlockTxIndex(index int) {
	SetETHBlockTxIndex(bs.db, index)
}

// GetETHBlockTxIndex 获取上次ETH监听到的区块里面的哪一笔交易
func (bs *BlockStore) GetETHBlockTxIndex() int {
	return GetETHBlockTxIndex(bs.db)
}

// GetBlockByHash 根据hash获取区块
func (bs *BlockStore) GetBlockByHash(blockID *crypto.Digest256) *pb.BlockPack {
	return GetCommitByID(bs.db, blockID)
}

// JustCommitIt 不做校验，直接保存区块
func (bs *BlockStore) JustCommitIt(blockPack *pb.BlockPack) {
	mu.Lock()
	JustCommitIt(bs.db, blockPack)
	mu.Unlock()
}

// SaveSnapshot 保存多签地址的快照和对应集群的快照
func (bs *BlockStore) SaveSnapshot(snapshot cluster.Snapshot) {
	SetMultiSigSnapshot(bs.db, cluster.MultiSigSnapshot.GetMultiSigInfos())
	multiSig, err := cluster.MultiSigSnapshot.GetLatestSigInfo()
	if err != nil {
		bsLogger.Debug("can't get multisig snapshot", "err", err)
		return
	}
	SetClusterSnapshot(bs.db, multiSig.BchAddress, snapshot)
	SetClusterSnapshot(bs.db, multiSig.BtcAddress, snapshot)
}

// GetMultiSigSnapshot 获取全量的多签地址快照
func (bs *BlockStore) GetMultiSigSnapshot() []cluster.MultiSigInfo {
	return GetMultiSigSnapshot(bs.db)
}

// GetClusterSnapshot 根据多签地址获取对应的集群快照
func (bs *BlockStore) GetClusterSnapshot(address string) *cluster.Snapshot {
	return GetClusterSnapshot(bs.db, address)
}

// HandleInitMsg 处理InitMsg
func (bs *BlockStore) HandleInitMsg(msg *pb.InitMsg) {
	deferredEvents := &task.Queue{}
	mu.Lock()
	bsLogger.Debug("get lock, begin handle init msg")
	bs.handleInitMsg(deferredEvents, msg)
	mu.Unlock()
	bsLogger.Debug("free lock, handle init msg done")
	deferredEvents.ExecAll()
}

// HandlePrepareMsg 处理PrepareMsg
func (bs *BlockStore) HandlePrepareMsg(msg *pb.PrepareMsg) {
	deferredEvents := &task.Queue{}
	mu.Lock()
	bsLogger.Debug("get lock, begin handle prepare msg")
	bs.handlePrepareMsg(deferredEvents, msg)
	mu.Unlock()
	bsLogger.Debug("free lock, handle prepare msg done")
	deferredEvents.ExecAll()
}

// HandleCommitMsg 处理CommitMsg
func (bs *BlockStore) HandleCommitMsg(msg *pb.CommitMsg) {
	deferredEvents := &task.Queue{}
	mu.Lock()
	bsLogger.Debug("get lock, begin handle commit msg")
	bs.handleCommitMsg(deferredEvents, msg)
	mu.Unlock()
	bsLogger.Debug("free lock, handle commit msg done")
	deferredEvents.ExecAll()
}

// HandleWeakAccuse 处理weak accuse，如果数量超过阈值，触发strong accuse
func (bs *BlockStore) HandleWeakAccuse(msg *pb.WeakAccuse) {
	deferredEvents := &task.Queue{}
	mu.Lock()
	bs.handleWeakAccuse(deferredEvents, msg)
	mu.Unlock()
	deferredEvents.ExecAll()
}

// HandleStrongAccuse 处理strong accuse, 提升term，重选leader
func (bs *BlockStore) HandleStrongAccuse(msg *pb.StrongAccuse) {
	deferredEvents := &task.Queue{}
	mu.Lock()
	bs.handleStrongAccuse(deferredEvents, msg)
	mu.Unlock()
	deferredEvents.ExecAll()
}

func (bs *BlockStore) HandleJoinRequest(msg *pb.JoinRequest) {
	deferredEvents := &task.Queue{}
	mu.Lock()
	bs.handleJoinRequest(deferredEvents, msg)
	mu.Unlock()
	deferredEvents.ExecAll()
}
func (bs *BlockStore) HandleJoinCheckSyncedRequest(msg *pb.JoinRequest) error {
	deferredEvents := &task.Queue{}
	mu.Lock()
	err := bs.handleJoinCheckSyncedRequest(deferredEvents, msg)
	deferredEvents.ExecAll()
	mu.Unlock()
	return err
}
func (bs *BlockStore) HandleLeaveRequest(msg *pb.LeaveRequest) {
	deferredEvents := &task.Queue{}
	mu.Lock()
	bs.handleLeaveRequest(deferredEvents, msg)
	mu.Unlock()
	deferredEvents.ExecAll()
}

// CommitBlockWithCheck commit新区块
func (bs *BlockStore) CommitBlockWithCheck(blockPack *pb.BlockPack) error {
	deferredEvents := &task.Queue{}
	err := bs.commitBlockWithCheck(deferredEvents, blockPack)
	if err == nil {
		deferredEvents.ExecAll()
	}
	return err
}

// GenSyncUpResponse 生成同步请求的返回
func (bs *BlockStore) GenSyncUpResponse(base int64, maxBlockN int64, needFresh bool) *pb.SyncUpResponse {
	return genSyncUpResponse(bs.db, base, maxBlockN, needFresh)
}

// HandleSignTx 处理交易加签请求，需要对交易做合法性校验以及重复签名的校验
func (bs *BlockStore) HandleSignTx(req *pb.SignRequest) {
	deferredEvents := &task.Queue{}
	start := time.Now().UnixNano()
	bs.handleSignReq(deferredEvents, req)
	end := time.Now().UnixNano()
	bsLogger.Debug("hanleSignTime", "time", (end-start)/1e6)
	deferredEvents.ExecAll()
}

func (bs *BlockStore) updateNodeTermWithInit(tasks *task.Queue, init *pb.InitMsg) bool {
	if init.Term <= GetNodeTerm(bs.db) {
		return false
	}

	if len(init.Votes) < cluster.QuorumN {
		return false
	}

	SetNodeTerm(bs.db, init.Term)
	tasks.Add(func() { bs.NewTermEvent.Emit(init.Term) })

	SetLastTermAccuse(bs.db, nil)
	SetWeakAccuses(bs.db, nil)
	SetFresh(bs.db, nil)
	return true
}

func (bs *BlockStore) handleInitMsg(tasks *task.Queue, init *pb.InitMsg) {
	switch {
	case init.Term > GetNodeTerm(bs.db):
		if init.Height > GetCommitHeight(bs.db)+1 {
			bsLogger.Debug("handleInitMsg trigger syncup", "init.height", init.Height,
				"commitheight", GetCommitHeight(bs.db))
			tasks.Add(func() { bs.NeedSyncUpEvent.Emit(init.NodeId) })
			return
		}
		if !bs.updateNodeTermWithInit(tasks, init) {
			return
		}
		fallthrough

	case init.Term == GetNodeTerm(bs.db):
		if init.Height != GetCommitHeight(bs.db)+1 {
			bs.handleWrongHeightProgressMsg(tasks, init)
			return
		}

		newFresh := pb.NewBlockPack(init)
		bsLogger.Debug("in handle init msg, begin validate txs")
		// allTxValid := bs.validateTxs(newFresh) todo
		allTxValid := Valid
		bsLogger.Debug("in handle init msg, validate txs done")

		reconfigValid := bs.checkReconfigBlock(newFresh)

		if !IsCommitted(bs.db, init.BlockId()) && IsConnectingTop(bs.db, newFresh) &&
			allTxValid == Valid && reconfigValid {
			// 仅在term有变更的情况下才去检查init消息里面的投票信息，因为这意味着有新的主节点, 需要检查合法性
			if init.Term > GetCommitTop(bs.db).Term() && !isInitSupportedByVotes(bs.db, init) {
				bsLogger.Debug("init msg is not supported by votes", "init", init)
				tasks.Add(func() { bs.NewWeakAccuseEvent.Emit(init.Term) })
				return
			}
			if prepareCache, ok := bs.prepareCache[init.Term]; ok {
				if cache, ok := prepareCache[init.Height]; ok {
					for _, msg := range cache {
						newFresh.Prepares[msg.NodeId] = msg
					}
					delete(bs.prepareCache[init.Term], init.Height)
				}
			}
			if commitCache, ok := bs.commitCache[init.Term]; ok {
				if cache, ok := commitCache[init.Height]; ok {
					for _, msg := range cache {
						newFresh.Commits[msg.NodeId] = msg
					}
					delete(bs.commitCache[init.Term], init.Height)
				}
			}
			SetFresh(bs.db, newFresh)
			tasks.Add(func() { bs.NewInitedEvent.Emit(newFresh) })
		} else if allTxValid == NotExist {
			bsLogger.Debug("validate txs has not exist tx")
			// tasks.Add(func() { bs.NeedSyncUpEvent.Emit(init.NodeId) })
			return
		} else {
			bsLogger.Warn("got invalid new fresh", "fresh", newFresh)
			tasks.Add(func() { bs.NewWeakAccuseEvent.Emit(init.Term) })
		}
	}
}

func (bs *BlockStore) handlePrepareMsg(tasks *task.Queue, msg *pb.PrepareMsg) {
	bsLogger.Debug("handle prepare msg", "msg.Term", msg.Term, "nodeTerm", GetNodeTerm(bs.db),
		"msg.Height", msg.Height, "commitHeight", GetCommitHeight(bs.db))
	switch {
	case msg.Term > GetNodeTerm(bs.db):
		bsLogger.Debug("handlePrepareMsg trigger syncup", "msg.term", msg.Term, "nodeterm", GetNodeTerm(bs.db))
		tasks.Add(func() { bs.NeedSyncUpEvent.Emit(msg.NodeId) })

	case msg.Term == GetNodeTerm(bs.db):
		if msg.Height != GetCommitHeight(bs.db)+1 {
			bs.handleWrongHeightProgressMsg(tasks, msg)
			return
		}
		fresh := GetFresh(bs.db)
		if fresh == nil {
			bsLogger.Debug("fresh is nil, save to cache")
			if _, ok := bs.prepareCache[msg.Term]; !ok {
				bs.prepareCache[msg.Term] = make(map[int64][]*pb.PrepareMsg)
			}
			bs.prepareCache[msg.Term][msg.Height] = append(bs.prepareCache[msg.Term][msg.Height], msg)
			// tasks.Add(func() { bs.NeedSyncUpEvent.Emit(msg.NodeId) })
			return
		}
		if !msg.BlockId.EqualTo(fresh.BlockId()) {
			bsLogger.Warn("handle prepare msg blockid not equal", "freshHeight", fresh.Height())
			return
		}
		if fresh.Prepares[msg.NodeId] != nil || len(fresh.Prepares) > cluster.QuorumN {
			//如果已经收到过这个节点的prepare或者收到的preapre已经足够多，则忽略，因为已经可以进行commit了
			bsLogger.Debug("handle prepare msg: already prepared", "prepares", fresh.Prepares)
			return
		}

		fresh.Prepares[msg.NodeId] = msg
		SetFresh(bs.db, fresh)

		bsLogger.Debug("prepareMsg handled", "preparedLen", len(fresh.Prepares), "QuorumN", cluster.QuorumN)
		if len(fresh.Prepares) >= cluster.QuorumN {
			updateVotie(bs.db, fresh)
			tasks.Add(func() { bs.NewPreparedEvent.Emit(fresh) })
		}
	}
}

func (bs *BlockStore) handleCommitMsg(tasks *task.Queue, msg *pb.CommitMsg) {
	switch {
	case msg.Term > GetNodeTerm(bs.db):
		bsLogger.Debug("handleCommitMsg trigger syncup", "msg.term", msg.Term, "nodeterm", GetNodeTerm(bs.db))
		tasks.Add(func() { bs.NeedSyncUpEvent.Emit(msg.NodeId) })
	case msg.Term == GetNodeTerm(bs.db):
		if msg.Height != GetCommitHeight(bs.db)+1 {
			bs.handleWrongHeightProgressMsg(tasks, msg)
			return
		}

		fresh := GetFresh(bs.db)
		if fresh == nil {
			bsLogger.Debug("fresh is nil, save to cache")
			if _, ok := bs.commitCache[msg.Term]; !ok {
				bs.commitCache[msg.Term] = make(map[int64][]*pb.CommitMsg)
			}
			bs.commitCache[msg.Term][msg.Height] = append(bs.commitCache[msg.Term][msg.Height], msg)
			return
		}
		if !msg.BlockId.EqualTo(fresh.BlockId()) {
			bsLogger.Warn("handle commit msg blockid not equal", "freshHeight", fresh.Height(), "msgHeight", msg.Height)
			return
		}
		if fresh.Commits[msg.NodeId] != nil {
			return
		}

		// assert.True(len(fresh.Commits) < cluster.QuorumN)
		fresh.Commits[msg.NodeId] = msg
		SetFresh(bs.db, fresh)

		bsLogger.Debug("commitMsg handled", "commitLen", len(fresh.Commits), "QuorumN", cluster.QuorumN)
		if len(fresh.Commits) >= cluster.QuorumN {
			assert.ErrorIsNil(bs.commitBlockWithCheck(tasks, fresh))
		}
	}
}

// weakAccuse的有效期 单位s
const accuseAvailableDuration = 60

func (bs *BlockStore) handleWeakAccuse(tasks *task.Queue, msg *pb.WeakAccuse) {
	switch {
	case msg.Term > GetNodeTerm(bs.db):
		bsLogger.Debug("weakaccuse trigger syncup", "msg.term", msg.Term, "nodeterm", GetNodeTerm(bs.db))
		tasks.Add(func() { bs.NeedSyncUpEvent.Emit(msg.NodeId) })
	case msg.Term == GetNodeTerm(bs.db):
		weakAccuses := GetWeakAccuses(bs.db)
		if weakAccuses.Size() >= cluster.AccuseQuorumN || weakAccuses.Get(msg.NodeId) != nil {
			return
		}
		// check删除已经过期的weakAccuse
		if weakAccuses != nil && len(weakAccuses.Accuses) > 0 {
			now := time.Now().Unix()
			for nodeID, accuse := range weakAccuses.Accuses {
				if now-accuse.Time > accuseAvailableDuration {
					delete(weakAccuses.Accuses, nodeID)
					bsLogger.Debug("accuse not available", "nodeID", nodeID, "term", accuse.Term)
				}
			}
		}
		weakAccuses.Set(msg.NodeId, msg)
		SetWeakAccuses(bs.db, weakAccuses)

		if weakAccuses.Size() >= cluster.AccuseQuorumN {
			tasks.Add(func() {
				bs.NewStrongAccuseEvent.Emit(pb.NewStrongAccuse(weakAccuses.Accuses))
			})
		}
	}
}

func (bs *BlockStore) handleStrongAccuse(tasks *task.Queue, msg *pb.StrongAccuse) {
	if msg.Term() >= GetNodeTerm(bs.db) {
		SetNodeTerm(bs.db, msg.Term()+1)
		tasks.Add(func() { bs.NewTermEvent.Emit(msg.Term() + 1) })

		SetLastTermAccuse(bs.db, msg)
		SetWeakAccuses(bs.db, nil)
		SetFresh(bs.db, nil)
		delete(bs.prepareCache, msg.Term())
		delete(bs.commitCache, msg.Term())
		tasks.Add(func() { bs.StrongAccuseProcessedEvent.Emit(msg) })
	}
}

// 对传过来的交易信息做校验，校验不通过广播accuse，通过则处理加签逻辑，并广播加签结果
// 仅有主节点能发送过来此类消息
// handleSignReq 处理签名请求
func (bs *BlockStore) handleSignReq(tasks *task.Queue, msg *pb.SignRequest) {
	bsLogger.Debug("begin handle sign tx msg")
	var signResult *pb.SignResult
	var err error
	targetChain := msg.GetWatchedEvent().GetTo()
	nodeTerm := GetNodeTerm(bs.db)

	switch {
	case msg.Term > nodeTerm:
		tasks.Add(func() { bs.NeedSyncUpEvent.Emit(msg.NodeId) })
		if targetChain == message.Bch || targetChain == message.Btc {
			signResult, err = pb.MakeSignResult(msg.WatchedEvent.GetBusiness(), pb.CodeType_NEEDSYNC, bs.localNodeId,
				msg.GetWatchedEvent().GetTxID(), nil, targetChain, nodeTerm, bs.signer)

			SetSignReq(bs.db, msg, msg.GetWatchedEvent().GetTxID())
			if err != nil {
				bsLogger.Error("sync make signedRes err", "err", err)
				return
			}
			tasks.Add(func() { bs.BroadcastSignResEvent.Emit(signResult) })
		}
	case msg.Term == nodeTerm:
		bsLogger.Debug("begin sign tx", "sctxid", msg.GetWatchedEvent().GetTxID())
		if targetChain == message.Bch || targetChain == message.Btc {
			var watcher *btcwatcher.MortgageWatcher
			if targetChain == message.Bch {
				watcher = bs.bchWatcher
			} else {
				watcher = bs.btcWatcher
			}

			//源链txID
			scTxID := msg.GetWatchedEvent().GetTxID()
			//所属业务
			business := msg.GetWatchedEvent().GetBusiness()

			buf := bytes.NewBuffer(msg.NewlyTx.Data)
			newlyTx := new(wire.MsgTx)
			newlyTx.Deserialize(buf)

			validateResult := bs.validateBtcSignReq(msg, newlyTx)
			if validateResult != validatePass {
				if validateResult == wrongInputOutput {
					bsLogger.Error("validate sign tx failed", "sctxid", scTxID)
					tasks.Add(func() { bs.NewWeakAccuseEvent.Emit(msg.Term) })
				} else {
					bsLogger.Debug("tx already signed", "sctxid", scTxID)
				}
				SetSignReq(bs.db, msg, scTxID)
				signResult, err = pb.MakeSignResult(business, pb.CodeType_REJECT, bs.localNodeId,
					scTxID, nil, targetChain, nodeTerm, bs.signer)
				if err != nil {
					bsLogger.Error("validatesign make signedResult err", "err", err)
					return
				}
				tasks.Add(func() { bs.BroadcastSignResEvent.Emit(signResult) })
				return
			}

			//签名前txID
			newlyTxId := newlyTx.TxHash().String()

			signStart := time.Now().UnixNano()
			sig, ok := watcher.SignTx(newlyTx, bs.signer.PubkeyHash)
			signEnd := time.Now().UnixNano()

			bsLogger.Debug("signtime", "scTxID", scTxID, "time", (signEnd-signStart)/1e6)
			if ok != 0 {
				bsLogger.Error("sign tx failed", "code", ok, "sctxid", scTxID)
				SetSignReq(bs.db, msg, scTxID)
				signResult, err = pb.MakeSignResult(business, pb.CodeType_REJECT, bs.localNodeId,
					scTxID, nil, targetChain, nodeTerm, bs.signer)
				if err != nil {
					bsLogger.Debug("sign fail make signedResult err", "err", err, "scTxID", scTxID)
					return
				}
				tasks.Add(func() { bs.BroadcastSignResEvent.Emit(signResult) })
				return
			}
			bsLogger.Debug("sign bch tx done", "sctxid", scTxID, "newlyTxid", newlyTxId)

			SetSignReq(bs.db, msg, scTxID)

			signResult, err := pb.MakeSignResult(business, pb.CodeType_SIGNED, bs.localNodeId,
				scTxID, sig, targetChain, nodeTerm, bs.signer)
			if err != nil {
				bsLogger.Debug("sign suc make signedResult err", "err", err, "scTxID", scTxID)
				return
			}
			tasks.Add(func() { bs.BroadcastSignResEvent.Emit(signResult) })
		} else if targetChain == message.Eth {
			scTxID := msg.GetWatchedEvent().GetTxID()
			business := msg.GetWatchedEvent().GetBusiness()
			//todo check eth
			_, err := bs.ethWatcher.SendTranxByInput(bs.signer.PubKeyHex, bs.signer.PubkeyHash, msg.NewlyTx.Data)
			if err != nil {
				bsLogger.Error("sign eth err", "err", err, "scTxID", scTxID)
				signRes, err := pb.MakeSignResult("", pb.CodeType_REJECT, bs.localNodeId, scTxID, nil, targetChain, nodeTerm, bs.signer)
				if err != nil {
					bsLogger.Debug("sign suc make signedResult err", "err", err, "scTxID", scTxID)
					return
				}
				tasks.Add(func() { bs.BroadcastSignResEvent.Emit(signRes) })
				return
			}
			bsLogger.Debug("sign eth tx done", "scTxID", scTxID)
			SetSignReq(bs.db, msg, scTxID)
			signRes, err := pb.MakeSignResult(business, pb.CodeType_SIGNED, bs.localNodeId, scTxID, nil, targetChain, nodeTerm, bs.signer)
			if err != nil {
				bsLogger.Debug("sign suc make signedResult err", "err", err, "scTxID", scTxID)
				return
			}
			tasks.Add(func() { bs.BroadcastSignResEvent.Emit(signRes) })
		}
	}

}

func (bs *BlockStore) handleJoinRequest(tasks *task.Queue, msg *pb.JoinRequest) {
	// 保存Join信息
	SetJoinNodeInfo(bs.db, msg)
	tasks.Add(func() { bs.OnJoinEvent.Emit(msg.Host) })
	go func(host string, db *dgwdb.LDBDatabase) {
		timer := time.NewTimer(2 * cluster.BlockInterval)
		for i := 0; i < 3; i++ {
			select {
			case <-timer.C:
				bsLogger.Debug("check weather joined", "host", host)
				if GetJoinNodeInfo(bs.db) == nil {
					return
				}
				// 超时都没有在最新的节点列表里面找到新节点的信息，发accuse
				bsLogger.Debug("join failed trigger weak accuse")
				bs.NewWeakAccuseEvent.Emit(GetNodeTerm(db))
			}
		}
		// 一直没有形成共识，也许节点并没有加入，清除加入的状态标记
		bs.JoinCancelEvent.Emit()
	}(msg.Host, bs.db)
}

func (bs *BlockStore) handleJoinCheckSyncedRequest(tasks *task.Queue, msg *pb.JoinRequest) error {
	height := msg.GetVote().GetVotie().Height
	//check 是否已经同步数据
	curHeight := GetCommitHeight(bs.db)
	if height < curHeight {
		return errors.New("not synced")
	}
	// 保存Join信息
	SetJoinNodeInfo(bs.db, msg)
	tasks.Add(func() { bs.OnJoinEvent.Emit(msg.Host) })
	go func(host string, db *dgwdb.LDBDatabase) {
		timer := time.NewTimer(2 * cluster.BlockInterval)
		for i := 0; i < 3; i++ {
			select {
			case <-timer.C:
				bsLogger.Debug("check weather joined", "host", host)
				for _, node := range cluster.NodeList {
					bsLogger.Debug("cluster info", "host", node.Url)
					if node.Url == host {
						return
					}
				}
				// 超时都没有在最新的节点列表里面找到新节点的信息，发accuse
				bsLogger.Debug("join failed trigger weak accuse")
				bs.NewWeakAccuseEvent.Emit(GetNodeTerm(db))
			}
		}
		// 一直没有形成共识，也许节点并没有加入，清除加入的状态标记
		bs.JoinCancelEvent.Emit()
	}(msg.Host, bs.db)
	return nil
}

func (bs *BlockStore) handleLeaveRequest(tasks *task.Queue, msg *pb.LeaveRequest) {
	SetLeaveNodeInfo(bs.db, msg)
	tasks.Add(func() { bs.OnLeaveEvent.Emit(msg.NodeId) })
	go func(nodeId int32, db *dgwdb.LDBDatabase) {
		timer := time.NewTimer(2 * cluster.BlockInterval)
		for i := 0; i < 3; i++ {
			select {
			case <-timer.C:
				if cluster.NodeList[nodeId].IsNormal {
					bs.NewWeakAccuseEvent.Emit(GetNodeTerm(db))
				} else {
					return
				}
			}
		}
		// 一直没有形成共识，也许节点并没有离开，恢复节点的状态
		bs.LeaveCancelEvent.Emit(nodeId)
	}(msg.NodeId, bs.db)
}

func (bs *BlockStore) handleWrongHeightProgressMsg(tasks *task.Queue, msg interface{}) {
	var (
		msgTerm    int64
		msgHeight  int64
		nodeId     int32
		msgBlockId *crypto.Digest256
	)

	switch m := msg.(type) {
	case *pb.InitMsg:
		msgTerm = m.Term
		msgHeight = m.Height
		msgBlockId = m.BlockId()
		nodeId = m.NodeId
	case *pb.PrepareMsg:
		msgTerm = m.Term
		msgHeight = m.Height
		msgBlockId = m.BlockId
		nodeId = m.NodeId
	case *pb.CommitMsg:
		msgTerm = m.Term
		msgHeight = m.Height
		msgBlockId = m.BlockId
		nodeId = m.NodeId
	default:
		bsLogger.Error("wrong msg type")
		return
	}

	assert.True(msgTerm == GetNodeTerm(bs.db))
	top := GetCommitTop(bs.db)
	assert.True(msgHeight != top.Height()+1)

	switch {
	case msgHeight == top.Height():
		if msgTerm > top.Term() && msgBlockId.EqualTo(top.BlockId()) {
			tasks.Add(func() { bs.CommittedInLowerTermEvent.Emit(msg) })
		}
	case msgHeight > top.Height()+1:
		bsLogger.Debug("wrongheight trigger syncup", "msgheight", msgHeight, "currheight", top.Height())
		tasks.Add(func() { bs.NeedSyncUpEvent.Emit(nodeId) })
	}
}

func updateVotie(db *dgwdb.LDBDatabase, candidate *pb.BlockPack) {
	assert.True(len(candidate.Prepares) >= cluster.QuorumN ||
		len(candidate.Commits) >= cluster.QuorumN)

	currentVotie := GetVotie(db)
	if currentVotie.HasLessTermHeightThan(candidate.ToVotie()) {
		SetVotie(db, candidate.ToVotie())
	}
}

// CommitSyncBlock 提交同步过来的区块
func (bs *BlockStore) CommitSyncBlock(blockPack *pb.BlockPack) error {
	if IsCommitted(bs.db, blockPack.BlockId()) {
		return fmt.Errorf("duplicated block id: %s", blockPack.BlockId().ToText())
	}
	if !IsConnectingTop(bs.db, blockPack) {
		return fmt.Errorf("the block is not connecting to the top of the current chain")
	}
	assert.True(blockPack.IsTxsBlock() || blockPack.IsReconfigBlock())
	blockPack.Prepares = nil
	JustCommitIt(bs.db, blockPack)
	bs.ts.OnNewBlockCommitted(blockPack)
	if blockPack.IsReconfigBlock() {

	}
	return nil
}

func (bs *BlockStore) commitBlockWithCheck(tasks *task.Queue, blockPack *pb.BlockPack) error {
	if IsCommitted(bs.db, blockPack.BlockId()) {
		return fmt.Errorf("duplicated block id: %s", blockPack.BlockId().ToText())
	}
	if len(blockPack.Commits) < cluster.QuorumN {
		return fmt.Errorf("not enough commits: %d", len(blockPack.Commits))
	}
	if !IsConnectingTop(bs.db, blockPack) {
		return fmt.Errorf("the block is not connecting to the top of the current chain")
	}

	assert.True(blockPack.IsTxsBlock() || blockPack.IsReconfigBlock())

	//oldTop := GetCommitTop(bs.db)
	newTop := blockPack.ShallowCopy()
	newTop.Prepares = nil
	JustCommitIt(bs.db, newTop)

	if newTop.Term() > GetNodeTerm(bs.db) {
		SetNodeTerm(bs.db, newTop.Term())
		tasks.Add(func() { bs.NewTermEvent.Emit(newTop.Term()) })
		SetLastTermAccuse(bs.db, nil)
		SetWeakAccuses(bs.db, nil)
	}

	SetFresh(bs.db, nil)
	updateVotie(bs.db, newTop)

	tasks.Add(func() { bs.NewCommittedEvent.Emit(newTop) })

	if blockPack.IsReconfigBlock() {
		// 如果是更新配置区块，则更新节点信息
		bsLogger.Debug("deal reconfig block")
		reconfig := blockPack.Block().Reconfig
		if reconfig.Type == pb.Reconfig_JOIN {
			joinInfo := GetJoinNodeInfo(bs.db)
			if joinInfo != nil {
				tasks.Add(func() { bs.JoinedEvent.Emit(reconfig.Host, reconfig.NodeId, joinInfo.Pubkey, joinInfo.Vote) })
			} else {
				bsLogger.Error("node join failed, joininfo not found")
			}
		} else {
			//cluster.DeleteNode(reconfig.NodeId)
			tasks.Add(func() { bs.LeavedEvent.Emit(reconfig.NodeId) })
		}
	}

	return nil
}

func isInitSupportedByVotes(db *dgwdb.LDBDatabase, init *pb.InitMsg) bool {
	if len(init.Votes) < cluster.QuorumN {
		return false
	}

	maxVotie := pb.GetMaxVotie(init.Votes)
	commitTop := GetCommitTop(db)
	return maxVotie.HasLessTermHeightThan(commitTop.ToVotie()) ||
		maxVotie.Block.Id.EqualTo(commitTop.BlockId()) ||
		maxVotie.Block.Id.EqualTo(init.BlockId())
}

func genSyncUpResponse(db *dgwdb.LDBDatabase, base int64, maxBlockN int64, needFresh bool) *pb.SyncUpResponse {
	rsp := new(pb.SyncUpResponse)
	commitHeight := GetCommitHeight(db)
	for h := base + 1; h <= commitHeight && h-base <= maxBlockN; h++ {
		rsp.Commits = append(rsp.Commits, GetCommitByHeight(db, h))
	}
	rsp.More = (commitHeight-base > maxBlockN)
	if !rsp.More && needFresh {
		rsp.Fresh = GetFresh(db)
		rsp.StrongAccuse = GetLastTermAccuse(db)
	}
	return rsp
}

func (bs *BlockStore) validateTxs(blockPack *pb.BlockPack) int {
	var checkChainTx []*pb.Transaction
	for _, tx := range blockPack.GetInit().Block.Txs {
		r := bs.ts.ValidateTx(tx)
		switch r {
		case Valid:
			continue
		case Invalid:
			return Invalid
		case NotExist:
			return NotExist
		case CheckChain:
			checkChainTx = append(checkChainTx, tx)
		default:
			return Invalid
		}
	}
	checkLen := len(checkChainTx)
	if checkLen > 0 {
		// 去链上校验交易会比较慢，交易量大的时候会导致交易时间过长。所以采用并行的方式去校验
		resultChan := make(chan int, checkLen)
		receivedCount := 0
		for _, tx := range checkChainTx {
			//todo review checkonchain 如何check eth交易
			/*
				go func(tx *pb.Transaction) {

					if tx.WatchedTx.To == "bch" {
						chainTx := bs.bchWatcher.GetTxByHash(tx.NewlyTxId)
						if chainTx == nil {
							resultChan <- Invalid
						} else {
							resultChan <- Valid
						}
					} else if tx.WatchedTx.To == "btc" {
						chainTx := bs.btcWatcher.GetTxByHash(tx.NewlyTxId)
						if chainTx == nil {
							resultChan <- Invalid
						} else {
							resultChan <- Valid
						}
					} else if tx.WatchedTx.To == "eth" {
						signMsg := GetSignMsg(bs.db, tx.WatchedTx.Txid)
						if signMsg == nil {
							bsLogger.Error("validate tx invalid, never signed", "sctxid", tx.WatchedTx.Txid)
							resultChan <- Invalid
							return
						}
						pushEvent, err := bs.ethWatcher.GetEventByHash(tx.NewlyTxId)
						if err != nil {
							bsLogger.Error("validate tx invalid, not found on chain", "sctxid", tx.WatchedTx.Txid)
							resultChan <- Invalid
						} else {
							if !bs.validateETHTx(pushEvent, tx.WatchedTx) {
								resultChan <- Invalid
							} else {
								resultChan <- Valid
							}
						}
					} else {
						resultChan <- Invalid
					}
				}(tx)
			*/
			go func(tx *pb.Transaction) {
				for _, pubtx := range tx.Vout {
					switch pubtx.Chain {
					case message.Bch:
						chainTx := bs.bchWatcher.GetTxByHash(pubtx.TxID)
						if chainTx == nil {
							resultChan <- Invalid
						} else {
							resultChan <- Valid
						}
					case message.Btc:
						chainTx := bs.btcWatcher.GetTxByHash(pubtx.TxID)
						if chainTx == nil {
							resultChan <- Invalid
						} else {
							resultChan <- Valid
						}
					case message.Eth:
						pushEvent, err := bs.ethWatcher.GetEventByHash(pubtx.TxID)
						if err != nil || pushEvent == nil {
							bsLogger.Error("validate tx invalid, not found on chain", "sctxid", tx.TxID)
							resultChan <- Invalid
						}
						//todo validate ethTx
						// } else {
						// 	if !bs.validateETHTx(pushEvent, tx.WatchedTx) {
						// 		resultChan <- Invalid
						// 	} else {
						// 		resultChan <- Valid
						// 	}
						// }
						resultChan <- Valid

					default:
						resultChan <- Invalid
					}
				}

			}(tx)
		}
		for {
			result := <-resultChan
			if result == Invalid {
				return Invalid
			}
			receivedCount++
			if receivedCount == checkLen {
				return Valid
			}
		}
	}
	return Valid
}

func (bs *BlockStore) validateETHTx(txInfo *ew.PushEvent, scTxInfo *pb.WatchedTxInfo) bool {
	if txInfo.Method != ew.VOTE_METHOD_MINT {
		return false
	}
	mintData, ok := txInfo.ExtraData.(*ew.ExtraMintData)
	if !ok {
		return false
	}
	return mintData.Proposal == scTxInfo.Txid && mintData.Wad == uint64(scTxInfo.RechargeList[0].Amount)
}

/*
func (bs *BlockStore) validateBtcSignTx(req *pb.SignTxRequest, newlyTx *wire.MsgTx) int {
	if req.WatchedTx.IsTransferTx() {
		return bs.validateTransferSignTx(req, newlyTx)
	}
	baseCheckResult := bs.baseCheck(req)
	if baseCheckResult != validatePass {
		return baseCheckResult
	}
	// 检查newlyTx里面的内容是否和watchdTx一致, 假定输出的顺序一致, 有可能会多出一个找零的输出
	if len(newlyTx.TxOut) != len(req.WatchedTx.RechargeList) &&
		len(newlyTx.TxOut) != len(req.WatchedTx.RechargeList)+1 &&
		len(newlyTx.TxOut) != len(req.WatchedTx.RechargeList)+2 {
		bsLogger.Warn("recharge check failed", "newcount", len(newlyTx.TxOut),
			"watchedcount", len(req.WatchedTx.RechargeList))
		return wrongInputOutput
	}
	for idx, recharge := range req.WatchedTx.RechargeList {
		txOut := newlyTx.TxOut[idx]
		outAddress := btcfunc.ExtractPkScriptAddr(txOut.PkScript, req.WatchedTx.To)
		if outAddress != recharge.Address || txOut.Value != recharge.Amount {
			bsLogger.Warn("recharge address or amount not equal", "outAddr", outAddress,
				"rechargeAddr", recharge.Address, "outAmount", txOut.Value, "rechargeAmount", recharge.Amount)
			return wrongInputOutput
		}
	}

	return validatePass
}
*/

/*
func (bs *BlockStore) validateTransferSignTx(req *pb.SignTxRequest, newlyTx *wire.MsgTx) int {
	if len(newlyTx.TxOut) != 1 || len(req.MultisigAddress) == 0 {
		bsLogger.Warn("txout check failed", "count", len(newlyTx.TxOut))
		return wrongInputOutput
	}

	var watcher *btcwatcher.MortgageWatcher
	if req.WatchedTx.To == "btc" {
		watcher = bs.btcWatcher
	} else {
		watcher = bs.bchWatcher
	}
	for _, txIn := range newlyTx.TxIn {
		utxoID := strings.Join([]string{txIn.PreviousOutPoint.Hash.String(), strconv.Itoa(int(txIn.PreviousOutPoint.Index))}, "_")
		utxoInfo := watcher.GetUtxoInfoByID(utxoID)
		if utxoInfo == nil || utxoInfo.Address != req.MultisigAddress {
			bsLogger.Error("txin check failed", "utxo", utxoInfo)
			return wrongInputOutput
		}
	}

	outAddress := btcfunc.ExtractPkScriptAddr(newlyTx.TxOut[0].PkScript, req.WatchedTx.To)
	if req.WatchedTx.To == "btc" {
		if outAddress == cluster.CurrMultiSig.BtcAddress {
			return validatePass
		}
		return wrongInputOutput
	}
	if outAddress == cluster.CurrMultiSig.BchAddress {
		return validatePass
	}
	return wrongInputOutput
}
*/
/*todo eth签名校验
func (bs *BlockStore) validateEthSignTx(req *pb.SignTxRequest) int {
	baseCheckResult := bs.baseCheck(req)
	if baseCheckResult != validatePass {
		return baseCheckResult
	}

	// 暂时只支持充值到一个地址
	if len(req.WatchedTx.RechargeList) != 1 {
		bsLogger.Warn("the count of output to eth is grater than 1")
		return wrongInputOutput
	}
	addredss := ew.HexToAddress(req.WatchedTx.RechargeList[0].Address)
	localInput, _ := bs.ethWatcher.EncodeInput(ew.VOTE_METHOD_MINT, req.WatchedTx.TokenTo, uint64(req.WatchedTx.RechargeList[0].Amount),
		addredss, req.WatchedTx.Txid)
	if !bytes.Equal(req.NewlyTx.Data, localInput) {
		bsLogger.Warn("verify eth input not passed", "sctxid", req.WatchedTx.Txid)
		return wrongInputOutput
	}
	return validatePass
}
*/
func (bs *BlockStore) validateEthSignReq(req *pb.SignRequest) int {
	baseCheckRes := bs.baseCheckSignReq(req)
	if baseCheckRes != validatePass {
		return baseCheckRes
	}
	return validatePass
}

/* todo
func (bs *BlockStore) validateWatchedTx(tx *pb.WatchedTxInfo) bool {
	var newTx *pb.WatchedTxInfo
	switch bs.ts.ValidateWatchedTx(tx) {
	case Valid:
		return true
	case Invalid:
		return false
	case NotExist:
		if tx.From == "bch" {
			chainTx := bs.bchWatcher.GetTxByHash(tx.Txid)
			if chainTx == nil {
				return false
			}
			newTx = pb.BtcToPbTx(chainTx.SubTx)
		} else if tx.From == "eth" {
			chainTx, err := bs.ethWatcher.GetEventByHash(tx.Txid)
			if err != nil {
				return false
			}
			newTx = pb.EthToPbTx(chainTx.ExtraData.(*ew.ExtraBurnData))
		} else if tx.From == "btc" {
			chainTx := bs.btcWatcher.GetTxByHash(tx.Txid)
			if chainTx == nil {
				return false
			}
			newTx = pb.BtcToPbTx(chainTx.SubTx)
		} else {
			return false
		}
		// 临时去链上获取的，可以马上存到缓存里面
		bs.ts.AddWatchedTx(newTx)
		return newTx.EqualTo(tx)
	default:
		return false
	}
}
*/

func (bs *BlockStore) validateBtcSignReq(req *pb.SignRequest, newlyTx *wire.MsgTx) int {
	if req.GetWatchedEvent().IsTransferEvent() {
		return bs.validateTransferSignReq(req, newlyTx)
	}
	baseCheckResult := bs.baseCheckSignReq(req)
	if baseCheckResult != validatePass {
		return baseCheckResult
	}
	//输出check 放到创建交易的地方
	return validatePass
}

func (bs *BlockStore) validateTransferSignReq(req *pb.SignRequest, newlyTx *wire.MsgTx) int {
	if len(newlyTx.TxOut) != 1 || len(req.MultisigAddress) == 0 {
		bsLogger.Warn("txout check failed", "count", len(newlyTx.TxOut))
		return wrongInputOutput
	}

	var watcher *btcwatcher.MortgageWatcher
	to := req.GetWatchedEvent().GetTo()
	var coinType string
	if to == message.Btc {
		coinType = "btc"
		watcher = bs.btcWatcher
	} else {
		coinType = "bch"
		watcher = bs.bchWatcher
	}
	for _, txIn := range newlyTx.TxIn {
		utxoID := strings.Join([]string{txIn.PreviousOutPoint.Hash.String(), strconv.Itoa(int(txIn.PreviousOutPoint.Index))}, "_")
		utxoInfo := watcher.GetUtxoInfoByID(utxoID)
		if utxoInfo == nil || utxoInfo.Address != req.MultisigAddress {
			bsLogger.Error("txin check failed", "utxo", utxoInfo)
			return wrongInputOutput
		}
	}

	outAddress := btcfunc.ExtractPkScriptAddr(newlyTx.TxOut[0].PkScript, coinType)
	if to == message.Btc {
		if outAddress == cluster.CurrMultiSig.BtcAddress {
			return validatePass
		}
		return wrongInputOutput
	}
	if outAddress == cluster.CurrMultiSig.BchAddress {
		return validatePass
	}
	return wrongInputOutput
}

// baseCheckSignReq 代替baseCheck
func (bs *BlockStore) baseCheckSignReq(req *pb.SignRequest) int {
	if !bs.validateWatchedEvent(req.GetWatchedEvent()) {
		bsLogger.Warn("watched event invalid", "scTxID", req.GetWatchedEvent().GetTxID())
		return wrongInputOutput
	}
	scTxID := req.GetWatchedEvent().GetTxID()
	// 以下步骤防止双花
	// 对应的交易已经达成共识或已经打包进区块了, 明显是重复交易
	scTxId := req.WatchedEvent.GetTxID()
	if bs.ts.IsTxInMem(scTxId) || bs.ts.HasTxInDB(scTxId) {
		bsLogger.Warn("watched tx in request have been done", "txid", scTxId)
		return alreadySigned
	}

	// 已经签过名了，拒绝重复签
	signReq := GetSignReq(bs.db, scTxID)
	if signReq != nil {
		bsLogger.Warn("request has been signed in this term", "scTxID", scTxID)
		return alreadySigned
	}
	return validatePass
}

// validateWatchedEvent 校验监听到的event 代替validateWatchedTx
func (bs *BlockStore) validateWatchedEvent(event *pb.WatchedEvent) bool {
	var newEvent *pb.WatchedEvent
	validateRes := bs.ts.ValidateWatchedEvent(event)
	switch validateRes {
	case Valid:
		return true
	case Invalid:
		return false
	case NotExist:
		//todo 到链上check
		return event.Equal(newEvent)
	default:
		return false
	}
}

func (bs *BlockStore) checkReconfigBlock(blockPack *pb.BlockPack) bool {
	if !blockPack.IsReconfigBlock() {
		return true
	}
	reconfig := blockPack.Block().Reconfig
	if reconfig == nil {
		return false
	}
	if reconfig.Type == pb.Reconfig_JOIN {
		msg := GetJoinNodeInfo(bs.db)
		return msg != nil && msg.Host == reconfig.Host && reconfig.NodeId == int32(cluster.TotalNodeCount)
	} else {
		msg := GetLeaveNodeInfo(bs.db)
		return msg != nil && msg.NodeId == reconfig.NodeId
	}
}
