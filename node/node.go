package node

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net"
	"os"
	"path"
	"sync"
	"time"

	"github.com/ofgp/common/defines"
	"github.com/ofgp/ofgp-core/accuser"
	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/config"
	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/dgwdb"
	"github.com/ofgp/ofgp-core/log"
	"github.com/ofgp/ofgp-core/message"
	"github.com/ofgp/ofgp-core/primitives"
	pb "github.com/ofgp/ofgp-core/proto"
	"github.com/ofgp/ofgp-core/util"
	"github.com/ofgp/ofgp-core/util/assert"

	btwatcher "swap/btwatcher"
	ew "swap/ethwatcher"
	// ew "github.com/ofgp/ethwatcher"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

const (
	syncUpBatchSize = 100
	maxSubscribers  = 100
	// 多签地址的转出交易，基本采用0确认，内存池中有交易即可
	defaultConfirmTolerance = 3 * time.Minute
	confirmTolerance        = 200
)

var (
	nodeLogger        = log.New("DEBUG", "node")
	errInvalidRequest = fmt.Errorf("invalid request")
	startMode         int
	BtcConfirms       int //check 交易确认数
	BchConfirms       int
	EthConfirms       int
	ConfirmTolerance  time.Duration
)

type waitingConfirmTx struct {
	msgId     string
	chainType string
	chainTxId string
	TokenTo   uint32
	timestamp time.Time
	inMem     bool
}

func (tx *waitingConfirmTx) setInMem() {
	tx.inMem = true
}

func (tx *waitingConfirmTx) isTimeout() bool {
	var confirmTolerance time.Duration
	if ConfirmTolerance == 0 {
		confirmTolerance = defaultConfirmTolerance
	} else {
		confirmTolerance = ConfirmTolerance * time.Minute
	}
	passed := confirmTolerance
	// if tx.chainType == "bch" {
	// 	passed = 10*time.Minute*time.Duration(BchConfirms) + confirmTolerance
	// } else if tx.chainType == "btc" {
	// 	passed = 10*time.Minute*time.Duration(BtcConfirms) + confirmTolerance
	// } else if tx.chainType == "eth" {
	// 	passed = 15*time.Second*time.Duration(EthConfirms) + confirmTolerance
	// }
	return time.Now().After(tx.timestamp.Add(passed))
}

// BraftNode node主结构, 也是程序启动的入口
type BraftNode struct {
	localNodeInfo cluster.NodeInfo
	signer        *crypto.SecureSigner
	blockStore    *primitives.BlockStore
	txStore       *primitives.TxStore
	peerManager   *cluster.PeerManager
	accuser       *accuser.Accuser
	leader        *Leader
	bchWatcher    *btwatcher.Watcher
	btcWatcher    *btwatcher.Watcher
	ethWatcher    *ew.Client
	syncDaemon    *SyncDaemon
	mu            sync.Mutex

	// signedTxs    sync.Map //已签名交易
	quit         context.CancelFunc
	isInReconfig bool

	signedResultChan  chan *pb.SignResult //处理sign结果
	signedResultCache sync.Map            //缓存签名结果
	pubsub            *pubServer          //与业务交互
	txInvoker         *txInvoker          //创建发送交易相关
	collectorFactory  *collectorFactory   //收集签名结果

	eventCh chan defines.PushEvent
}

func getFederationAddress() cluster.MultiSigInfo {
	var err error
	pubkeyList := cluster.GetPubkeyList()
	btcFedAddress, btcRedeem, err := btwatcher.GetMultiSigAddress(pubkeyList, cluster.QuorumN, "btc")
	assert.ErrorIsNil(err)
	bchFedAddress, bchRedeem, err := btwatcher.GetMultiSigAddress(pubkeyList, cluster.QuorumN, "bch")
	assert.ErrorIsNil(err)
	multiSig := cluster.MultiSigInfo{
		BtcAddress:      btcFedAddress,
		BtcRedeemScript: btcRedeem,
		BchAddress:      bchFedAddress,
		BchRedeemScript: bchRedeem,
	}
	// cluster.AddMultiSigInfo(multiSig)
	return multiSig
}

const defaultUtxoLockTime = 60

const defaultTxConnPoolSize = 100
const defaultBlockConnPoolSize = 20

// NewBraftNode 生成&启动一个node对象并返回
func NewBraftNode(localNodeInfo cluster.NodeInfo) *BraftNode {
	dgwConf := config.GetDGWConf()
	keyStoreConf := config.GetKeyStoreConf()

	db, newlyCreated := OpenDbOrDie(dgwConf.DBPath, "braftdb")
	if newlyCreated {
		nodeLogger.Debug("initializing new db")
		primitives.InitDB(db, primitives.GenesisBlockPack)
	}

	initWatchHeight(db)

	signer := cluster.NodeSigners[localNodeInfo.Id]

	privateKey := keyStoreConf.KeyStorePrivateKey
	serviceID := keyStoreConf.ServiceID
	url := keyStoreConf.URL
	signer.InitKeystoreParam(privateKey, serviceID, url)

	// 从db还原历史的多签快照
	multiSigList := primitives.GetMultiSigSnapshot(db)
	cluster.SetMultiSigSnapshot(multiSigList)

	// bchFederationAddress, bchRedeem, btcFederationAddress, btcRedeem := getFederationAddress()
	multiSig := getFederationAddress()
	nodeLogger.Debug("get multisig address", "btc", multiSig.BtcAddress, "bch", multiSig.BchAddress)
	cluster.SetCurrMultiSig(multiSig)

	var (
		// btcWatcher *btcwatcher.MortgageWatcher
		// bchWatcher *btcwatcher.MortgageWatcher
		btcWatcher *btwatcher.Watcher
		bchWatcher *btwatcher.Watcher
		ethWatcher *ew.Client
		err        error
	)
	if startMode != cluster.ModeWatch && startMode != cluster.ModeTest {
		utxoLockTime := dgwConf.UtxoLockTime
		if utxoLockTime == 0 {
			utxoLockTime = defaultUtxoLockTime
		}
		bchWatcher, err = btwatcher.NewWatcher(defines.CHAIN_CODE_BCH)
		// bchWatcher, err = btcwatcher.NewMortgageWatcher("bch", dgwConf.BchHeight,
		// 	multiSig.BchAddress, multiSig.BchRedeemScript, utxoLockTime)
		if err != nil {
			panic(fmt.Sprintf("new bitcoin watcher failed, err: %v", err))
		}
		// btcWatcher, err = btcwatcher.NewMortgageWatcher("btc", dgwConf.BtcHeight,
		// 	multiSig.BtcAddress, multiSig.BtcRedeemScript, utxoLockTime)
		btcWatcher, err = btwatcher.NewWatcher(defines.CHAIN_CODE_BTC)
		if err != nil {
			panic(fmt.Sprintf("new bitcoin watcher failed, err: %v", err))
		}
		nodeConf := dgwConf.Nodes[localNodeInfo.Id]
		ethWatcher, err = ew.NewEthWatcher(dgwConf.EthClientURL,
			dgwConf.EthConfirmCount, nodeConf.Pubkey)
		if err != nil {
			panic(fmt.Sprintf("new eth watcher failed, err: %v", err))
		}
	}

	ts := primitives.NewTxStore(db)

	//事件监听ch
	eventCh := make(chan defines.PushEvent)

	bs := primitives.NewBlockStore(db, ts, btcWatcher, bchWatcher, ethWatcher, signer, localNodeInfo.Id, eventCh)

	var txInvoker *txInvoker
	var collectorFactory *collectorFactory
	if startMode != cluster.ModeWatch && startMode != cluster.ModeTest {
		ethOp := newEthOperator(ethWatcher, bs, signer)
		bchOp := newBchOprator(bchWatcher)
		btcOp := newBtcOprator(btcWatcher)
		txInvoker = newTxInvoker(ethOp, bchOp, btcOp)

		bchCollector := newBchResCollector(bchWatcher)
		btcCollector := newBtcResCollector(btcWatcher)
		ethCollector := newEthResCollector()
		collectorFactory = newCollectorFactory(bchCollector, btcCollector, ethCollector)

	}
	//交易相关连接池大小
	txConnPoolSize := dgwConf.TxConnPoolSize
	if txConnPoolSize == 0 {
		txConnPoolSize = defaultTxConnPoolSize
	}
	//区块相关PoolSize
	blockPoolSize := dgwConf.BlockConnPoolSize
	if blockPoolSize == 0 {
		blockPoolSize = defaultBlockConnPoolSize
	}
	pm := cluster.NewPeerManager(localNodeInfo.Id, txConnPoolSize, blockPoolSize)

	ac := accuser.NewAccuser(localNodeInfo, signer, pm)
	ld := NewLeader(localNodeInfo, bs, ts, signer, btcWatcher, bchWatcher, ethWatcher, pm, txInvoker)
	syncDaemon := NewSyncDaemon(db, bs, pm)

	bn := &BraftNode{
		localNodeInfo: localNodeInfo,
		signer:        signer,
		blockStore:    bs,
		txStore:       ts,
		peerManager:   pm,
		accuser:       ac,
		leader:        ld,
		bchWatcher:    bchWatcher,
		btcWatcher:    btcWatcher,
		ethWatcher:    ethWatcher,
		syncDaemon:    syncDaemon,
		mu:            sync.Mutex{},
		isInReconfig:  false,

		signedResultChan: make(chan *pb.SignResult),
		pubsub:           newPubServer(1),
		txInvoker:        txInvoker,
		collectorFactory: collectorFactory,
		eventCh:          eventCh,
	}
	//重新添加监听列表
	// if len(multiSigList) > 0 {
	bn.changeFederationAddrs(multiSig, multiSigList)
	// }

	bs.NeedSyncUpEvent.Subscribe(func(nodeId int32) {
		nodeLogger.Debug("Need Syncup", "from", nodeId)
		syncDaemon.SignalSyncUp(nodeId)
	})

	ld.NewInitEvent.Subscribe(func(init *pb.InitMsg) {
		go func() {
			bs.HandleInitMsg(init)
		}()
	})

	bs.NewInitedEvent.Subscribe(func(fresh *pb.BlockPack) {
		if fresh.Init.NodeId == localNodeInfo.Id {
			// the node is leader, only leader can create init msg and broadcast it
			pm.Broadcast(fresh.Init, true, false)
		}
		nodeLogger.Debug("Inited", "height", fresh.Height())
		prepare, err := pb.MakePrepareMsg(fresh.BlockInfo(), localNodeInfo.Id, signer)
		if err != nil {
			nodeLogger.Error("make prepare err", "err", err, "height", fresh.Height())
			return
		}
		bs.HandlePrepareMsg(prepare)
		pm.Broadcast(prepare, true, false)
	})

	bs.NewPreparedEvent.Subscribe(func(fresh *pb.BlockPack) {
		nodeLogger.Debug("Prepared", "height", fresh.Height())
		commit, err := pb.MakeCommitMsg(fresh.BlockInfo(), localNodeInfo.Id, signer)
		if err != nil {
			nodeLogger.Error("make commit err", "err", err, "height", fresh.Height())
			return
		}
		bs.HandleCommitMsg(commit)
		pm.Broadcast(commit, true, false)
	})

	bs.NewCommittedEvent.Subscribe(func(newTop *pb.BlockPack) {
		nodeLogger.Info("Committed", "term", newTop.Term(), "height", newTop.Height())
		//删除waitingconfir tx
		bn.onNewBlockCommitted(newTop)
		ts.OnNewBlockCommitted(newTop)
		ac.OnNewCommitted(newTop)
		//通知commit事件
		bn.pubCommit(newTop)
	})

	bs.CommittedInLowerTermEvent.Subscribe(func(msg interface{}) {
		nodeLogger.Debug("CommittedInLowerTerm", "msg", msg)
		switch m := msg.(type) {
		case *pb.PrepareMsg:
			prepare, err := pb.MakePrepareMsg(m.BlockInfoLite(), localNodeInfo.Id, signer)
			assert.ErrorIsNil(err)
			pm.NotifyPrepareMsg(m.NodeId, prepare)
		case *pb.CommitMsg:
			commit, err := pb.MakeCommitMsg(m.BlockInfoLite(), localNodeInfo.Id, signer)
			assert.ErrorIsNil(err)
			pm.NotifyCommitMsg(m.NodeId, commit)
		}
	})

	bs.NewWeakAccuseEvent.Subscribe(func(term int64) {
		primitives.SetAccuseRecord(db, term, localNodeInfo.Id, cluster.LeaderNodeOfTerm(term), 1, "bs accuse")
		ac.TriggerByBlockStore(term)
	})

	bs.NewStrongAccuseEvent.Subscribe(func(sc *pb.StrongAccuse) {
		nodeLogger.Debug("new strong accuse formed and is broadcasting", "strong accuse", sc.DebugString())
		primitives.SetAccuseRecord(db, sc.Term(), localNodeInfo.Id, cluster.LeaderNodeOfTerm(sc.Term()), 2, "")
		bs.HandleStrongAccuse(sc)
		pm.Broadcast(sc, true, false)
	})

	bs.StrongAccuseProcessedEvent.Subscribe(func(sc *pb.StrongAccuse) {
		nodeLogger.Debug("strong accuse reveived and processed", "strong accuse", sc.DebugString())
		ts.OnTermChanged(sc.Term() + 1)
		ac.OnTermChange(sc.Term() + 1)
	})

	bs.ReconfigEvent.Subscribe(func() {
		// Reconfig过程中，需要暂停交易处理
		bn.isInReconfig = true
	})

	bs.BroadcastSignResEvent.Subscribe(func(msg *pb.SignResult) {
		ts.DeleteWaitSign(msg.ScTxID)
		pm.Broadcast(msg, false, false)
	})

	bs.OnJoinEvent.Subscribe(func(host string) {
		ld.OnNewNodeJoin(host)
	})

	bs.JoinedEvent.Subscribe(func(host string, nodeId int32, pubkey string, vote *pb.Vote) {
		bs.DeleteJoinNodeInfo()
		ld.OnNodeJoinedDone(vote)
		cluster.AddMultiSigInfo(cluster.CurrMultiSig)
		snapShot := cluster.GetSnapshot()
		bs.SaveSnapshot(snapShot)
		for {
			// 确保老的交易都已经处理完毕
			if ts.HasWaitSignTx() || ld.hasTxToSign {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			break
		}
		cluster.AddNode(host, nodeId, pubkey, "")
		multiSig := getFederationAddress()
		nodeLogger.Debug("new multisig address", "btc", multiSig.BtcAddress, "bch", multiSig.BchAddress)
		btcWatcher.ChangeFederationAddress(multiSig.BtcAddress, multiSig.BtcRedeemScript)
		bchWatcher.ChangeFederationAddress(multiSig.BchAddress, multiSig.BchRedeemScript)
		cluster.SetCurrMultiSig(multiSig)

		// 调用ETH的网关合约接口，增加合约成员
		address, _, _ := ew.GetAddressFromPub(pubkey)
		proposal := "J_" + host + pubkey
		_, err := ethWatcher.GatewayTransaction(signer.PubKeyHex, signer.PubkeyHash, ew.VOTE_METHOD_ADDVOTER, address, proposal)
		if err != nil {
			nodeLogger.Error("add voter to contract failed", "err", err)
		}

		pm.AddNode(cluster.NodeList[nodeId])

		saveNewConfig(nodeId)
		bn.isInReconfig = false
	})

	bs.JoinCancelEvent.Subscribe(func() {
		ld.OnJoinCancel()
	})

	bs.OnLeaveEvent.Subscribe(func(nodeId int32) {
		//删除之前创建集群snapshot
		cluster.CreateSnapShot()
		cluster.DeleteNode(nodeId)
		ld.OnNodeLeave(nodeId)
	})

	bs.LeavedEvent.Subscribe(func(nodeId int32) {

		bs.DeleteLeaveNodeInfo()
		ld.OnNodeLeaveDone()
		//将当前多签地址加入监听列表
		cluster.AddMultiSigInfo(cluster.CurrMultiSig)
		//节点离开标记节点不可用
		snapshot := cluster.ClusterSnapshot
		if snapshot == nil { //没有接收到leave请求
			nodeLogger.Debug("leave snapshot not found", "leavingNodeId", ld.leavingNodeId)
			snapshot = cluster.CreateSnapShot()
		}
		if int(nodeId) < len(snapshot.NodeList) {
			snapshot.NodeList[nodeId].IsNormal = false
			bs.SaveSnapshot(*snapshot)
		} else {
			nodeLogger.Debug("leave nodeId wrong", "nodeId", nodeId, "size", len(snapshot.NodeList))
		}

		for {
			// 确保老的交易都已经处理完毕
			if ts.HasWaitSignTx() || ld.hasTxToSign {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			break
		}
		// 为了防止有节点没有收到LeaveRequest，共识成功后再做一次删除操作
		cluster.DeleteNode(nodeId)
		cluster.DelSnapShot()
		// 调用ETH的网关合约接口，删掉合约成员
		pubkey := hex.EncodeToString(cluster.NodeList[nodeId].PublicKey)
		address, _, _ := ew.GetAddressFromPub(pubkey)
		proposal := "L_" + cluster.NodeList[nodeId].Url + pubkey
		_, err := ethWatcher.GatewayTransaction(signer.PubKeyHex, signer.PubkeyHash, ew.VOTE_METHOD_REMOVEVOTER, address, proposal)
		if err != nil {
			nodeLogger.Error("remove voter from contract failed", "err", err)
		}
		//节点离开修改多签地址
		multiSig := getFederationAddress()
		nodeLogger.Debug("leave new multisig address", "btc", multiSig.BtcAddress, "bch", multiSig.BchAddress)
		btcWatcher.ChangeFederationAddress(multiSig.BtcAddress, multiSig.BtcRedeemScript)
		bchWatcher.ChangeFederationAddress(multiSig.BchAddress, multiSig.BchRedeemScript)
		cluster.SetCurrMultiSig(multiSig)

		saveNewConfig(nodeId)
	})

	bs.LeaveCancelEvent.Subscribe(func(nodeId int32) {
		cluster.RecoverNode(nodeId)
		// 仅仅只是为了消除leader的leave nodeid标记
		ld.OnNodeLeaveDone()
	})

	ld.BecomeLeaderEvent.Subscribe(func(nodeInfo cluster.NodeInfo, term int64) {
		ld.beComeLeaderCnt++
		nodeLogger.Info("become leader", "term", term)
	})

	ld.RetireEvent.Subscribe(func(nodeInfo cluster.NodeInfo, term int64) {
		nodeLogger.Debug("Retire", "term", term)
	})

	ts.TxOverdueEvent.Subscribe(func(term int64) {
		nodeLogger.Info("weak accuse by txstore")
		primitives.SetAccuseRecord(db, term, localNodeInfo.Id, cluster.LeaderNodeOfTerm(term), 1, "tx accuse")
		ac.TriggerByTxStore(term)
	})

	return bn
}

func (bn *BraftNode) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	bn.quit = cancel
	nodeLogger.Debug("begin run node", "startMode", startMode, "watchMode", cluster.ModeWatch)

	go bn.txStore.Run(ctx)
	if startMode != cluster.ModeWatch && startMode != cluster.ModeTest {
		go bn.accuser.Run(ctx)
		go bn.leader.Run(ctx)

		go bn.saveSignedResult(ctx)
		go bn.runCheckSignTimeout(ctx)
		go bn.runCheckConfirm(ctx)
	}
	if startMode != cluster.ModeTest {
		go bn.syncDaemon.Run(ctx)
	}

	if startMode != cluster.ModeWatch && startMode != cluster.ModeTest {
		bn.accuser.OnTermChange(bn.blockStore.GetNodeTerm()) // init accuser's term
		go bn.watchNewTx(ctx)
		go bn.voteDaemon(ctx)
		//TODO 改用通知的方式获取交易是否确认
		// go bn.watchWatingConfirmTx(ctx)
	}
	if startMode != cluster.ModeTest {
		go bn.regularSyncUp(ctx)
	}
}

func OpenDbOrDie(dbPath, subPath string) (db *dgwdb.LDBDatabase, newlyCreated bool) {
	if len(dbPath) == 0 {
		homeDir, err := util.GetHomeDir()
		if err != nil {
			panic("Cannot detect the home dir for the current user.")
		}
		dbPath = path.Join(homeDir, subPath)
	}

	fmt.Println("open db path ", dbPath)
	info, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		if err := os.Mkdir(dbPath, 0700); err != nil {
			panic(fmt.Errorf("Cannot create db path %v,err:%v", dbPath, err))
		}
		newlyCreated = true
	} else {
		if err != nil {
			panic(fmt.Errorf("Cannot get info of %v", dbPath))
		}
		if !info.IsDir() {
			panic(fmt.Errorf("Datavse path (%v) is not a directory", dbPath))
		}
		if c, _ := ioutil.ReadDir(dbPath); len(c) == 0 {
			newlyCreated = true
		} else {
			newlyCreated = false
		}
	}

	db, err = dgwdb.NewLDBDatabase(dbPath, cluster.DbCache, cluster.DbFileHandles)
	if err != nil {
		panic(fmt.Errorf("Failed to open database at %v", dbPath))
	}
	return
}

func (bn *BraftNode) voteDaemon(ctx context.Context) {
	newTermChan := make(chan int64)
	bn.blockStore.NewTermEvent.Subscribe(func(term int64) {
		newTermChan <- term
	})
	timerInterval := 3 * time.Second
	timer := time.NewTimer(timerInterval)

	for {
		select {
		case <-newTermChan:
		case <-timer.C:
		case <-ctx.Done():
			return
		}

		var (
			vote *pb.Vote
			err  error
		)
		if bn.blockStore.GetFresh() == nil {
			nodeTerm := bn.blockStore.GetNodeTerm()
			commitTop := bn.blockStore.GetCommitTop()
			if commitTop.Height() == 0 || nodeTerm > commitTop.Term() {
				vote, err = pb.MakeVote(nodeTerm, bn.blockStore.GetVotie(),
					bn.localNodeInfo.Id, bn.signer)
				if err != nil {
					timer.Reset(timerInterval)
					continue
				}
			}
		}

		if vote != nil {
			bn.peerManager.NotifyVote(cluster.LeaderNodeOfTerm(vote.Term), vote)
			timer.Reset(timerInterval)
		} else {
			// 如果当前term已经不需要投票了，就暂停timer触发，除非进入到新的term
			timer.Stop()
		}
	}
}

// just for robustness
func (bn *BraftNode) regularSyncUp(ctx context.Context) {
	var timer <-chan time.Time
	if startMode != cluster.ModeWatch {
		timer = time.Tick(60 * time.Second)
	} else {
		timer = time.Tick(5 * time.Second)
	}
	for {
		select {
		case <-timer:
			perm := rand.Perm(len(cluster.NodeList))
			numSync := len(cluster.NodeList)
			if numSync > 5 {
				numSync = 5
			}
			for i := 0; i < numSync; i++ {
				if !cluster.NodeList[perm[i]].IsNormal {
					continue
				}
				bn.syncDaemon.SignalSyncUp(cluster.NodeList[perm[i]].Id)
			}
		case <-ctx.Done():
			return
		}
	}
}

/*
todo 铸币熔币相关check
func (bn *BraftNode) checkSubTx(tx *btwatcher.SubTransaction) bool {
	if !bn.ethWatcher.VerifyAppInfo(tx.From, tx.TokenFrom, tx.TokenTo) {
		nodeLogger.Warn("verify app info not passed", "scTxid", tx.ScTxid)
		return false
	}
	return true
}
*/

func newWatchedEvent(event defines.PushEvent) *pb.WatchedEvent {
	return &pb.WatchedEvent{
		Business:  event.GetBusiness(),
		EventType: event.GetEventType(),
		TxID:      event.GetTxID(),
		Amount:    event.GetAmount(),
		Fee:       event.GetFee(),
		From:      uint32(event.GetFrom()),
		To:        uint32(event.GetTo()),
		Data:      event.GetData(),
		Proposal:  event.GetProposal(),
	}
}

func (bn *BraftNode) getTxPosition(confHeight int64, confIndex int, chain uint8) (height int64, index int) {
	if confHeight == 0 {
		bchPosition := bn.blockStore.GetTxPosition(defines.CHAIN_CODE_BCH)
		height = bchPosition.GetHeight()
		index = int(bchPosition.GetIndex())
	} else {
		height = confHeight
		index = confIndex
	}

	return
}

// 后面可能会改成每条链一个goroutine，如果每条链的交易量都很大，一个select可能处理不过来
func (bn *BraftNode) watchNewTx(ctx context.Context) {
	dgwConf := config.GetDGWConf()

	eventCh := bn.eventCh
	bchHeight, bchIndex := bn.getTxPosition(dgwConf.BchHeight, dgwConf.BchTranIx, defines.CHAIN_CODE_BCH)
	nodeLogger.Debug("bch watch info", "height", bchHeight, "index", bchIndex)
	bn.bchWatcher.StartWatch(bchHeight, int(bchIndex), eventCh)
	btcHeight, btcIndex := bn.getTxPosition(dgwConf.BtcHeight, dgwConf.BtcTranIx, defines.CHAIN_CODE_BTC)
	nodeLogger.Debug("btc watch info", "height", btcHeight, "index", btcIndex)
	bn.btcWatcher.StartWatch(btcHeight, int(btcIndex), eventCh)

	ethHeight, ethIndex := bn.getTxPosition(dgwConf.EthHeight, dgwConf.EthTranIdx, defines.CHAIN_CODE_ETH)
	nodeLogger.Debug("eth watch info", "height", ethHeight, "index", ethIndex)
	bn.ethWatcher.StartWatch(*big.NewInt(ethHeight), int(ethIndex), eventCh)
	for event := range eventCh {
		if event.GetBusiness() != "" {
			nodeLogger.Debug("receive event", "scTxID", event.GetTxID(), "chain", event.GetFrom(), "type", event.GetEventType(), "business", event.GetBusiness(), "to", event.GetTo())
		}

		// 防止重复发布事件
		if event.GetBusiness() != "" && bn.pubsub.hasTopic(event.GetBusiness()) && !bn.txStore.IsWatched(event.GetTxID()) && !bn.txStore.HasTxInDB(event.GetTxID()) {
			watchedEvent := newWatchedEvent(event)
			bn.txStore.AddWatchedEvent(watchedEvent)
			bn.pubWatcherEvent(watchedEvent)
			//删除已签名标记 停止confirmTimeout check
			if event.GetEventType() == defines.EVENT_P2P_SWAP_CONFIRM {
				scTxID := event.GetProposal()
				bn.onTxConfirmed(scTxID)
			}
		}
		//设置监听高度
		bn.blockStore.SetTxPosition(event.GetFrom(), &pb.WatchedTxPosition{
			Height: event.GetHeight(),
			Index:  int64(event.GetTranxIx()),
		})
	}
	// bchTxChan := bn.bchWatcher.GetTxChan()
	// btcTxChan := bn.btcWatcher.GetTxChan()

	// ethEventChan := make(chan *ew.PushEvent)
	// height := bn.blockStore.GetETHBlockHeight()
	// index := bn.blockStore.GetETHBlockTxIndex()
	// if height == nil {
	// 	dgwConf := config.GetDGWConf()
	// 	h := dgwConf.EthHeight
	// 	height = big.NewInt(h)
	// 	index = dgwConf.EthTranIdx
	// }

	// bn.ethWatcher.StartWatch(*height, index, ethEventChan)

	// for {
	// 	if bn.isInReconfig {
	// 		time.Sleep(10 * time.Millisecond)
	// 		continue
	// 	}
	// 	watchedEvent = nil
	// 	select {
	// 	case event := <-bchEventChan:
	// 		nodeLogger.Debug("receive bch event", "event", event)
	// 		watchedEvent = newWatchedEvent(event)

	// 	case event := <-btcEventChan:
	// 		nodeLogger.Debug("receive btc event", "event", event)
	// 		watchedEvent = newWatchedEvent(event)
	// 	case ev := <-ethEventChan:
	// 		//todo eth event
	// 		bn.dealEthEvent(ev)
	// 	case <-ctx.Done():
	// 		return
	// 	}
	// 	if watchedEvent != nil {
	// 		bn.txStore.AddWatchedEvent(watchedEvent)
	// 		bn.pubWatcherEvent(watchedEvent)
	// 	}
	// }
}

/*
func (bn *BraftNode) dealEthEvent(ev *ew.PushEvent) {
	switch ev.Method {
	case ew.TOKEN_METHOD_BURN:
		// 熔币事件
		burnData := ev.ExtraData.(*ew.ExtraBurnData)
		nodeLogger.Debug("receive eth burn", "tx", burnData.ScTxid)
		watchedTx := pb.EthToPbTx(burnData)
		if watchedTx != nil {
			//todo 监听事件
			// bn.txStore.AddWatchedTx(watchedTx)
		} else {
			nodeLogger.Debug("create watchedTx fail", "tx", burnData.ScTxid)
		}
	case ew.VOTE_METHOD_MINT:
		// 铸币结果通知事件
		if (ev.Events & ew.VOTE_TX_MINT) == 0 {
			// nodeLogger.Debug("receive eth vote", "tx", ev.Tx.TxHash.Hex())
			return
		}
		mintData := ev.ExtraData.(*ew.ExtraMintData)
		nodeLogger.Debug("receive eth create", "tx", ev.Tx.TxHash.Hex(), "mintData", mintData.AppCode)
		//todo 交易确认
		// go func(scTxId string) {
		// 	bn.mu.Lock()
		// 	bn.txStore.CreateInnerTx(ev.Tx.TxHash.Hex(), scTxId)
		// 	delete(bn.waitingConfirmTxs, mintData.Proposal)
		// 	bn.mu.Unlock()
		// }(mintData.Proposal)
	case ew.VOTE_METHOD_ADDVOTER:
		if (ev.Events & ew.VOTE_TX_VOTERADDED) == 0 {
			return
		}
		nodeLogger.Debug("contract voter added")
	case ew.VOTE_METHOD_REMOVEVOTER:
		if (ev.Events & ew.VOTE_TX_VOTERREMOVED) == 0 {
			return
		}
		nodeLogger.Debug("contract voter removed")
	}
	// 保存ETH监听的高度和高度内的交易索引
	bn.blockStore.SetETHBlockHeight(ev.Tx.BlockNumber)
	bn.blockStore.SetETHBlockTxIndex(ev.Tx.TxIndex)
}
*/

// markTxSigned 标记已签名交易
func (bn *BraftNode) markTxSigned(msg *message.SignedMsg) {
	bn.txStore.AddSigned(msg)
}

func (bn *BraftNode) onNewBlockCommitted(pack *pb.BlockPack) {
	block := pack.Block()
	if block != nil {
		if len(block.Txs) > 0 {
			bn.mu.Lock()
			defer bn.mu.Unlock()
			for _, tx := range block.Txs { //删除已签名标记
				for _, pubtx := range tx.Vin {
					bn.signedResultCache.Delete(pubtx.GetTxID())
					bn.txStore.DelSigned(pubtx.GetTxID())
				}
				// for _, pubtx := range tx.Vout {
				// 	bn.signedResultCache.Delete(pubtx.GetTxID())
				// 	bn.txStore.DelSigned(pubtx.GetTxID())
				// }
			}
		}
	}
}

// onTxConfirmed tx confirmed清理缓存
func (bn *BraftNode) onTxConfirmed(scTxID string) {
	bn.signedResultCache.Delete(scTxID)
	bn.txStore.DelSigned(scTxID)
	bn.txStore.DelCreateSignCache(scTxID)
	bn.txStore.DeleteWaitSign(scTxID)
	bn.txStore.DeleteWatchedEvent(scTxID)
}

//TODO
// deleteFromWaiting 删除已签名标记 在收到confirm通知的时后调用
func (bn *BraftNode) deleteFromSigned(scTxID string) {
	bn.mu.Lock()
	defer bn.mu.Unlock()
	nodeLogger.Debug("delete from signed", "scTxID", scTxID)
	bn.txStore.DelSigned(scTxID)
}

/*
func (bn *BraftNode) checkTxOnChain(tx *waitingConfirmTx, wg *sync.WaitGroup) {
	defer wg.Done()
	hash := tx.msgId
	// 已经发送出去的交易，超时不引起任何accuse，仅打印日志记录。因为有可能是链上拥堵
	if tx.isTimeout() && !tx.inMem {
		nodeLogger.Debug("has timeout tx", "sctxid", tx.msgId)
		bn.deleteFromWaiting(tx.msgId)
		signReqMsg := bn.blockStore.GetSignReq(hash)
		if signReqMsg != nil {
			bn.clearOnFail(signReqMsg)
		}
		return
	}
	if tx.chainType == "bch" {
		nodeLogger.Debug("begin filter bch tx", "sctxid", tx.msgId)
		chainTx := bn.bchWatcher.GetTxByHash(tx.chainTxId)
		if chainTx != nil {
			if !tx.inMem {
				tx.setInMem()
			}
			if chainTx.Confirmations >= uint64(BchConfirms) {
				bn.txStore.CreateInnerTx(chainTx.ScTxid, tx.msgId)
				bn.deleteFromWaiting(tx.msgId)
			}
		}
	} else if tx.chainType == "btc" {
		chainTx := bn.btcWatcher.GetTxByHash(tx.chainTxId)
		if chainTx != nil {
			if !tx.inMem {
				tx.setInMem()
			}
			if chainTx.Confirmations >= uint64(BtcConfirms) {
				bn.txStore.CreateInnerTx(chainTx.ScTxid, tx.msgId)
				bn.deleteFromWaiting(tx.msgId)
			}
		}

	}
}
*/

/*
func (bn *BraftNode) getWaitingTxCh() (<-chan *waitingConfirmTx, int) {
	bn.mu.Lock()
	defer bn.mu.Unlock()

	txSize := len(bn.waitingConfirmTxs)
	txCh := make(chan *waitingConfirmTx, txSize)

	for _, tx := range bn.waitingConfirmTxs {
		txCh <- tx
	}
	close(txCh)

	return txCh, txSize
}
*/
//链上验证并发数
/*
var CheckOnChainCur int

//探测时间间隔
var CheckOnChainInterval time.Duration

func (bn *BraftNode) watchWatingConfirmTx(ctx context.Context) {
	if CheckOnChainInterval == 0 {
		CheckOnChainInterval = 30
	}
	if CheckOnChainCur == 0 {
		CheckOnChainCur = 5
	}
	timer := time.NewTicker(CheckOnChainInterval * time.Second)
	for {
		select {
		case <-timer.C:

			// nodeLogger.Debug("watching confirm tx", "count", len(tmp))
			//get waitingcheck tx
			txCh, txSize := bn.getWaitingTxCh()
			if txSize == 0 {
				break
			}

			//等待check 完毕
			wg := new(sync.WaitGroup)
			wg.Add(txSize)
			for i := 0; i < CheckOnChainCur; i++ {
				go func() {
					for tx := range txCh {
						bn.checkTxOnChain(tx, wg)
					}
				}()
			}
			wg.Wait()
			nodeLogger.Debug("watching confirm tx done")
		case <-ctx.Done():
			return
		}
	}
}
*/

// Stop 节点停止运行
func (bn *BraftNode) Stop() {
	bn.quit()
}

// LeaveCluster 先广播LeaveRequest，然后再停止运行
func (bn *BraftNode) LeaveCluster() {
	nodeLogger.Warn("ready to leave cluster", "nodeid", bn.localNodeInfo.Id)
	msg, err := pb.MakeLeaveRequest(bn.localNodeInfo.Id, "", bn.signer)
	if err != nil {
		nodeLogger.Error("make leave message failed")
		return
	}
	bn.peerManager.Broadcast(msg, true, false)
	bn.quit()
}

func initWatchHeight(db *dgwdb.LDBDatabase) {
	height := primitives.GetCurrentHeight(db, "bch")
	if height > 0 {
		viper.Set("DGW.bch_height", height)
	}
	height = primitives.GetCurrentHeight(db, "eth")
	if height > 0 {
		viper.Set("DGW.eth_height", height)
	}
}

func getRemoteClusterNodes(host string) *pb.NodeList {
	conn, err := grpc.Dial(host, grpc.WithInsecure())
	if err != nil {
		return nil
	}
	defer conn.Close()
	client := pb.NewBraftClient(conn)
	nodeList, err := client.GetClusterNodes(context.Background(), new(pb.Void))
	if err != nil {
		return nil
	}
	return nodeList
}

// getFederationAddressUsePubkeys 根据pubkey获取多签地址
func getFederationAddressUsePubkeys(pubKeys []string, quorumN int) cluster.MultiSigInfo {
	btcFedAddress, btcRedeem, err := btwatcher.GetMultiSigAddress(pubKeys, quorumN, "btc")
	assert.ErrorIsNil(err)
	bchFedAddress, bchRedeem, err := btwatcher.GetMultiSigAddress(pubKeys, quorumN, "bch")
	assert.ErrorIsNil(err)
	multiSig := cluster.MultiSigInfo{
		BtcAddress:      btcFedAddress,
		BtcRedeemScript: btcRedeem,
		BchAddress:      bchFedAddress,
		BchRedeemScript: bchRedeem,
	}
	return multiSig
}

type JoinMsg struct {
	LocalID       int32
	MultiSigInfos []cluster.MultiSigInfo
}

// InitJoin 根据引导节点做集群配置信息的初始化
func InitJoin() *JoinMsg {
	dgwConf := config.GetDGWConf()
	keyStoreConf := config.GetKeyStoreConf()
	initHost := dgwConf.InitNodeHost
	joinMsg := new(JoinMsg)
	if len(initHost) == 0 {
		joinMsg.LocalID = -1
		return joinMsg
	}

	nodeList := getRemoteClusterNodes(initHost)

	//引导节点snapshot multiSig
	for _, multiSig := range nodeList.MultiSigInfoList {
		joinMsg.MultiSigInfos = append(joinMsg.MultiSigInfos, cluster.MultiSigInfo{
			BtcAddress:      multiSig.BtcAddress,
			BtcRedeemScript: multiSig.BtcRedeemScript,
			BchAddress:      multiSig.BchAddress,
			BchRedeemScript: multiSig.BchRedeemScript,
		})
	}

	//待加入集群的multisig
	multiSig := getFederationAddressUsePubkeys(nodeList.GetPubkeys(), int(nodeList.QuorumN))
	joinMsg.MultiSigInfos = append(joinMsg.MultiSigInfos, multiSig)

	nodeLogger.Debug("init join get multisig address", "btc", multiSig.BtcAddress, "bch", multiSig.BchAddress)
	cluster.InitWithNodeList(nodeList)

	//创建当前集群的快照
	cluster.CreateSnapShot()

	cluster.AddNode(dgwConf.LocalHost, int32(len(nodeList.NodeList)), dgwConf.LocalPubkey,
		keyStoreConf.LocalPubkeyHash)
	localID := int32(len(nodeList.NodeList))
	saveNewConfig(localID)
	joinMsg.LocalID = localID
	return joinMsg
}

// saveNewConfig 保存最新的配置信息到viper，以及持久化到配置文件
func saveNewConfig(localId int32) {
	// 保存新的节点信息到config file
	// viper.Set("KEYSTORE.count", cluster.TotalNodeCount)
	// viper.Set("DGW.count", cluster.TotalNodeCount)
	// viper.Set("DGW.local_id", localId)
	// // 下次启动就是以正常模式启动
	// if startMode == cluster.ModeJoin {
	// 	viper.Set("DGW.start_mode", 1)
	// }
	// for i, nodeInfo := range cluster.NodeList {
	// 	viper.Set("KEYSTORE.key_"+strconv.Itoa(i), hex.EncodeToString(nodeInfo.PublicKey))
	// 	viper.Set("DGW.host_"+strconv.Itoa(i), nodeInfo.Url)
	// 	viper.Set("DGW.status_"+strconv.Itoa(i), nodeInfo.IsNormal)
	// }
	// viper.Set("DGW.new_node_host", "")
	// viper.Set("DGW.new_node_pubkey", "")
	confs := make(map[string]interface{})
	confs["KEYSTORE.count"] = cluster.TotalNodeCount
	confs["DGW.count"] = cluster.TotalNodeCount
	confs["DGW.local_id"] = localId
	if startMode == cluster.ModeJoin {
		confs["DGW.start_mode"] = 1
	}
	nodeConfs := make([]map[string]interface{}, 0)
	for _, nodeInfo := range cluster.NodeList {
		nodeConf := map[string]interface{}{
			"host":   nodeInfo.Url,
			"status": nodeInfo.IsNormal,
			"pubkey": hex.EncodeToString(nodeInfo.PublicKey),
		}
		nodeConfs = append(nodeConfs, nodeConf)
	}
	confs["dgw.nodes"] = nodeConfs
	confs["DGW.new_node_host"] = ""
	confs["DGW.new_node_pubkey"] = ""
	config.Set(confs)
	//viper.WriteConfig()
}

// sendJoinRequest 给网关的所有节点广播Join请求
func (bn *BraftNode) sendJoinRequest() {
	dgwConf := config.GetDGWConf()
	localHost := dgwConf.LocalHost
	localPubkey := dgwConf.LocalPubkey

	for _, nodeInfo := range cluster.NodeList {
		if nodeInfo.Id == bn.localNodeInfo.Id {
			return
		}
		if !nodeInfo.IsNormal {
			continue
		}
		go func(host string) {
			conn, err := grpc.Dial(host, grpc.WithInsecure())
			if err != nil {
				nodeLogger.Error("make connection failed", "to", host, "err", err)
				return
			}
			defer conn.Close()
			client := pb.NewBraftClient(conn)
			vote, err := pb.MakeVote(bn.blockStore.GetNodeTerm(), bn.blockStore.GetVotie(), bn.localNodeInfo.Id, bn.signer)
			if err != nil {
				nodeLogger.Error("make vote failed", "err", err)
				return
			}
			nodeLogger.Debug("join vote", "vote", vote.DebugString())
			msg, err := pb.MakeJoinRequest(localHost, localPubkey, vote, bn.signer)
			if err != nil {
				nodeLogger.Error("make join request failed", "err", err)
				return
			}
			client.NotifyJoin(context.Background(), msg)
		}(nodeInfo.Url)
	}
}

//create join req
func (bn *BraftNode) createJoinReq(host, pubkey string, signer *crypto.SecureSigner) (*pb.JoinRequest, error) {
	curBlockPack := bn.blockStore.GetCommitTop()
	if curBlockPack == nil {
		return nil, errors.New("get top block nil")
	}
	term := curBlockPack.Term()
	votie := curBlockPack.ToVotie()
	vote, err := pb.MakeVote(term, votie, bn.localNodeInfo.Id, signer)
	if err != nil {
		return nil, err
	}
	msg, err := pb.MakeJoinRequest(host, pubkey, vote, signer)
	if err != nil {
		return nil, err
	}
	return msg, err
}

func (bn *BraftNode) syncBeforeSendJoinReq(localID int32) {
	var syncedCnt int
	finished := make(map[int32]struct{})
	for syncedCnt < cluster.ClusterSnapshot.QuorumN {
		for _, node := range cluster.NodeList {
			if node.Id != localID {
				if _, ok := finished[node.Id]; !ok {
					err := bn.syncDaemon.doJoinSyncUp(node.Id)
					if err != nil {
						continue
					}
				}
				finished[node.Id] = struct{}{}
				syncedCnt++
			}
		}
		time.Sleep(time.Second)
	}
}

// sendJoinCheckSyncedRequest 给网关的所有节点广播Join请求  直到与QuorumN个节点的数据保持同步
func (bn *BraftNode) sendJoinCheckSyncedRequest() {
	dgwConf := config.GetDGWConf()
	localHost := dgwConf.LocalHost
	localPubkey := dgwConf.LocalPubkey
	var syncedCnt int
	syncedNode := make(map[int32]struct{})

	for syncedCnt < cluster.ClusterSnapshot.QuorumN {
		for _, nodeInfo := range cluster.NodeList {
			if nodeInfo.Id == bn.localNodeInfo.Id {
				continue
			}
			if !nodeInfo.IsNormal {
				nodeLogger.Warn("node is not normal", "node", nodeInfo)
				continue
			}

			if _, ok := syncedNode[nodeInfo.Id]; ok {
				nodeLogger.Debug("node has been synced", "node", nodeInfo)
				continue
			}

			//创建同步client
			host := nodeInfo.Url
			conn, err := grpc.Dial(host, grpc.WithInsecure())
			if err != nil {
				nodeLogger.Error("make connection failed", "to", host, "err", err)
				continue
			}
			defer conn.Close()
			client := pb.NewBraftClient(conn)
			//request param
			msg, err := bn.createJoinReq(localHost, localPubkey, bn.signer)
			if err != nil {
				nodeLogger.Error("make join request failed", "err", err)
				continue
			}
			joinRes, err := client.NotifyJoinCheckSynced(context.Background(), msg)
			nodeLogger.Debug("res from joinreq", "res", joinRes)
			if err != nil {
				nodeLogger.Error("sync err", "host:", host, "err", err)
				continue
			}
			if joinRes == nil {
				nodeLogger.Error("node joinRes err", "host", host, "res", joinRes)
				continue
			}
			if !joinRes.Synced { //未同步完成
				err := bn.syncDaemon.doJoinSyncUp(joinRes.NodeID)
				if err != nil {
					nodeLogger.Error("sync form host err", "host", host, "err", err)
					continue
				}
			} else { //同步完成
				nodeLogger.Info("sync finished", "node", nodeInfo.Id)
				syncedCnt++
				syncedNode[joinRes.NodeID] = struct{}{}
			}
		}
		time.Sleep(time.Second)
	}
	nodeLogger.Info("sync suc")
}

func checkJoinSuccess() bool {
	dgwConf := config.GetDGWConf()
	checkHost := dgwConf.InitNodeHost
	localHost := dgwConf.LocalHost
	for i := 0; i < 20; i++ {
		nodeList := getRemoteClusterNodes(checkHost)
		for _, node := range nodeList.NodeList {
			if node.Host == localHost && node.IsNormal {
				return true
			}
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

//添加多签地址和redmeScript
func (bn *BraftNode) changeFederationAddrs(latest cluster.MultiSigInfo, multiSigs []cluster.MultiSigInfo) {
	if len(multiSigs) > 0 {
		for _, multiSig := range multiSigs {
			nodeLogger.Debug("init old multisig address", "btc", multiSig.BtcAddress, "bch", multiSig.BchAddress)
			bn.btcWatcher.ChangeFederationAddress(multiSig.BtcAddress, multiSig.BtcRedeemScript)
			bn.bchWatcher.ChangeFederationAddress(multiSig.BchAddress, multiSig.BchRedeemScript)
		}
	}
	//设置最新的multiSig
	if latest.BchAddress != "" && latest.BtcAddress != "" {
		nodeLogger.Debug("init latest multisig address", "btc", latest.BtcAddress, "bch", latest.BchAddress)
		//set btc
		bn.btcWatcher.ChangeFederationAddress(latest.BtcAddress, latest.BtcRedeemScript)
		bn.btcWatcher.SetCurrentFederationAddress(latest.BtcAddress, latest.BtcRedeemScript)

		//set bch
		bn.bchWatcher.ChangeFederationAddress(latest.BchAddress, latest.BchRedeemScript)
		bn.bchWatcher.SetCurrentFederationAddress(latest.BchAddress, latest.BchRedeemScript)
	}
}

// SubScribe 订阅business
func (bn *BraftNode) SubScribe(business string) chan BusinessEvent {
	ch := bn.pubsub.subScribe(business)
	return ch
}

// RunNew 启动server
func RunNew(id int32, multiSigInfos []cluster.MultiSigInfo) (*grpc.Server, *BraftNode) {
	dgwConf := config.GetDGWConf()
	startMode = dgwConf.StartMode
	p2pPort := dgwConf.LocalP2PPort
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", p2pPort))
	if err != nil {
		panic(fmt.Sprintf("failed to listen: %v", err))
	}

	// 默认的流控大小为64K，改成1M和10M
	grpcServer := grpc.NewServer(grpc.InitialWindowSize(1048576), grpc.InitialConnWindowSize(10485760))
	braftNode := NewBraftNode(cluster.NodeList[id])
	nodeLogger.Debug("begin run braft node", "bchconfirm", BchConfirms, "ethconfirm", EthConfirms)
	pb.RegisterBraftServer(grpcServer, braftNode)
	go func() {
		grpcServer.Serve(lis)
	}()

	if startMode == cluster.ModeJoin {
		//start sync
		braftNode.syncBeforeSendJoinReq(id)
		//send join request that check synced
		braftNode.sendJoinCheckSyncedRequest()
		if !checkJoinSuccess() {
			panic("join cluster failed")
		}
		cluster.SetMultiSigSnapshot(multiSigInfos)
		braftNode.blockStore.SaveSnapshot(*cluster.ClusterSnapshot)

		latestMultiSig := getFederationAddress()

		cluster.SetCurrMultiSig(latestMultiSig)
		braftNode.changeFederationAddrs(latestMultiSig, multiSigInfos)
	}
	// braftNode.Run()

	return grpcServer, braftNode
}
