package node

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	ew "swap/ethwatcher"

	"github.com/ofgp/common/defines"
	"github.com/ofgp/ofgp-core/primitives"
	pb "github.com/ofgp/ofgp-core/proto"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/crypto"
	dgwLog "github.com/ofgp/ofgp-core/log"

	log "github.com/inconshreveable/log15"
)

var apiLog log.Logger

const firstBlockHeight = 0
const defaultCreateUsed = 10

func init() {
	apiLog = dgwLog.New("debug", "http_api")
}

func (node *BraftNode) GetBlockHeight() int64 {
	return node.blockStore.GetCommitHeight()
}

func (node *BraftNode) GetBlockInfo(height int64) *pb.BlockInfo {
	blockPack := node.blockStore.GetCommitByHeight(height)
	return blockPack.BlockInfo()
}

func (node *BraftNode) GetTxBySidechainTxId(scTxId string) *primitives.TxQueryResult {
	return node.txStore.QueryTxInfoBySidechainId(scTxId)
}

// AddWatchedEvent 添加监听事件
func (node *BraftNode) AddWatchedEvent(event *pb.WatchedEvent) error {
	if !node.txStore.HasWatchedEvent(event) {
		node.txStore.AddWatchedEvent(event)
	} else {
		return errors.New("tx already exist")
	}
	return nil
}

//blcokView for api
type BlockView struct {
	Height      int64       `json:"height"`
	ID          string      `json:"id"`
	PreID       string      `json:"pre_id"`
	TxCnt       int         `json:"tx_cnt"`
	Txs         []*TxViewV1 `json:"txs"`
	Time        int64       `json:"time"`         //unix 时间戳
	Size        int         `json:"size"`         //块大小
	CreatedUsed int64       `json:"created_used"` //块被创建花费的时间
	Miner       string      `json:"miner"`        //产生块的服务器
}

type TxView struct {
	FromTxHash  string   `json:"from_tx_hash"` //转出txhash
	ToTxHash    string   `json:"to_tx_hash"`   //转入txhash
	DGWTxHash   string   `json:"dgw_hash"`     //dgwtxhash
	From        string   `json:"from"`         //转出链
	To          string   `json:"to"`           //转入链
	Time        int64    `json:"time"`
	Block       string   `json:"block"`        //所在区块blockID
	BlockHeight int64    `json:"block_height"` //所在区块高度
	Amount      int64    `json:"amount"`
	ToAddrs     []string `json:"to_addrs"`
	FromFee     int64    `json:"from_fee"`
	DGWFee      int64    `json:"dgw_fee"`
	ToFee       int64    `json:"to_fee"`
	TokenCode   uint32   `json:"token_code"`
	AppCode     uint32   `json:"app_code"`
}

// PubTx 公网tx
type PubTx struct {
	Chain  uint32 `json:"chain"`
	TxID   string `json:"tx_id"`
	Amount int64  `json:"amount"`
	Code   uint32 `json:"code"`
}

// TxViewV1 tx json
type TxViewV1 struct {
	Block       string   `json:"block"`        //所在区块blockID
	BlockHeight int64    `json:"block_height"` //所在区块高度
	DGWFee      int64    `json:"dgw_fee"`      // 网关手续费
	TxID        string   `json:"tx_id"`
	Business    string   `json:"business"`
	Vin         []*PubTx `json:"vin"`
	Vout        []*PubTx `json:"vout"`
	Time        int64    `json:"time"`
}

func getHexString(digest *crypto.Digest256) string {
	if digest == nil || len(digest.Data) == 0 {
		return ""
	}
	return hex.EncodeToString(digest.Data)
}

func getPubTxs(txs []*pb.PublicTx) []*PubTx {
	var pubTxs []*PubTx
	for _, tx := range txs {
		pubTx := &PubTx{
			Chain:  tx.GetChain(),
			TxID:   tx.GetTxID(),
			Amount: tx.GetAmount(),
			Code:   tx.GetCode(),
		}
		pubTxs = append(pubTxs, pubTx)
	}

	return pubTxs
}
func (node *BraftNode) createTxViewV1(blockID string, height int64, tx *pb.Transaction) *TxViewV1 {
	if tx == nil {
		apiLog.Error(fmt.Sprintf("block:%s transaciton is nil", blockID))
		return nil
	}
	txView := &TxViewV1{
		Block:       blockID,
		BlockHeight: height,
		TxID:        getHexString(tx.TxID),
		Business:    tx.GetBusiness(),
		Vin:         getPubTxs(tx.Vin),
		Vout:        getPubTxs(tx.Vout),
		Time:        tx.GetTime(),
	}
	return txView
}

func getChain(chain string) uint32 {
	switch chain {
	case "bch":
		return uint32(defines.CHAIN_CODE_BCH)
	case "eth":
		return uint32(defines.CHAIN_CODE_ETH)
	case "btc":
		return uint32(defines.CHAIN_CODE_BTC)
	}
	return 0
}

// tx类型修改
func (node *BraftNode) createTxView(blockID string, height int64, tx *pb.TransactionOld) *TxViewV1 {
	if tx == nil {
		apiLog.Error(fmt.Sprintf("block:%s transaciton is nil", blockID))
		return nil
	}
	watchedTx := tx.GetWatchedTx()
	if watchedTx == nil {
		apiLog.Error(fmt.Sprintf("block:%s has no watched tx", blockID))
		return nil
	}
	addrList := watchedTx.GetRechargeList()
	addrs := make([]string, 0)
	for _, addr := range addrList {
		addrs = append(addrs, addr.Address)
	}
	// var tokenCode, appCode uint32
	// if watchedTx.From == "eth" { //熔币
	// 	tokenCode, appCode = watchedTx.TokenTo, watchedTx.TokenFrom
	// } else { //铸币
	// 	tokenCode, appCode = watchedTx.TokenFrom, watchedTx.TokenTo
	// }
	// txView := &TxView{
	// 	FromTxHash:  watchedTx.GetTxid(),
	// 	DGWTxHash:   getHexString(tx.GetId()),
	// 	ToTxHash:    tx.NewlyTxId,
	// 	From:        watchedTx.From,
	// 	To:          watchedTx.To,
	// 	Block:       blockID,
	// 	BlockHeight: height,
	// 	Amount:      watchedTx.Amount,
	// 	FromFee:     watchedTx.GetFee(),
	// 	ToAddrs:     addrs,
	// 	Time:        tx.Time,
	// 	TokenCode:   tokenCode,
	// 	AppCode:     appCode,
	// }

	txView := &TxViewV1{
		Block:       blockID,
		BlockHeight: height,
		TxID:        getHexString(tx.GetId()),
		Business:    "mint", //todo 铸币熔币业务name
		Vin: []*PubTx{
			&PubTx{
				Chain:  getChain(watchedTx.From),
				TxID:   watchedTx.GetTxid(),
				Amount: watchedTx.Amount,
				Code:   watchedTx.TokenFrom,
			},
		},
		Vout: []*PubTx{
			&PubTx{
				Chain:  getChain(watchedTx.To),
				TxID:   tx.NewlyTxId,
				Amount: watchedTx.Amount,
				Code:   watchedTx.TokenTo,
			},
		},
		Time: tx.Time,
	}
	return txView
}
func (node *BraftNode) createBlockView(createdUsed int64, blockPack *pb.BlockPack) *BlockView {
	block := blockPack.Block()
	var txViews []*TxViewV1
	if block == nil {
		return nil
	}
	var height int64
	var nodeID int32
	height = blockPack.Height()
	if blockPack.GetInit() != nil {
		nodeID = blockPack.GetInit().GetNodeId()
	}
	peerNode := node.peerManager.GetNode(nodeID)
	var miner string
	if peerNode != nil {
		miner = peerNode.Name
	}
	txViews = make([]*TxViewV1, 0)
	if len(block.TxOlds) > 0 {
		txs := block.TxOlds
		blockID := getHexString(block.GetId())
		for _, tx := range txs {
			txView := node.createTxView(blockID, height, tx)
			if txView != nil {
				txViews = append(txViews, txView)
			} else {
				apiLog.Error("create txview err", "transaction", tx)
			}
		}
	}
	if len(block.Txs) > 0 {
		txs := block.Txs
		blockID := getHexString(block.GetId())
		for _, tx := range txs {
			txView := node.createTxViewV1(blockID, height, tx)
			if txView != nil {
				txViews = append(txViews, txView)
			} else {
				apiLog.Error("create txview err", "transaction", tx)
			}
		}
	}

	bw := &BlockView{
		Height:      height,
		ID:          getHexString(block.Id),
		PreID:       getHexString(block.PrevBlockId),
		TxCnt:       len(txViews),
		Txs:         txViews,
		Time:        int64(block.TimestampMs / 1000),
		Size:        blockPack.XXX_Size(),
		CreatedUsed: createdUsed,
		Miner:       miner,
	}
	return bw
}

// GetBlockCurrent 获取最新区块
func (node *BraftNode) GetBlockCurrent() *BlockView {
	curHeight := node.blockStore.GetCommitHeight()
	return node.GetBlockBytHeight(curHeight)
}

// GetBlocks 获取区块 start 开始高度 end结束高度[start,end)
func (node *BraftNode) GetBlocks(start, end int64) []*BlockView {
	if end <= start {
		apiLog.Error(fmt.Sprintf("end:%d is less than start:%d", start, end))
		return nil
	}
	var beiginFromFirstBlock bool
	if start < firstBlockHeight {
		apiLog.Error("start < firstBlockHeight")
		return nil
	}
	if start == firstBlockHeight {
		beiginFromFirstBlock = true
	}
	if start > firstBlockHeight { //大于创世块
		start = start - 1
	}

	blockPacks := node.blockStore.GetCommitsByHeightSec(start, end)
	size := len(blockPacks)
	if size == 0 {
		return nil
	}
	blcokViews := make([]*BlockView, 0)
	if beiginFromFirstBlock { //==创世块
		blockView := node.createBlockView(0, blockPacks[0])
		blcokViews = append(blcokViews, blockView)
	}
	pre := blockPacks[0]

	for i := 1; i < size; i++ {
		blcokPack := blockPacks[i]
		if blcokPack == nil {
			continue
		}
		var createdUsed int64
		if pre.TimestampMs() == 0 {
			createdUsed = defaultCreateUsed
		} else {
			createdUsed = blcokPack.TimestampMs()/1000 - pre.TimestampMs()/1000
		}
		blockView := node.createBlockView(createdUsed, blcokPack)
		blcokViews = append(blcokViews, blockView)
		pre = blcokPack
	}

	return blcokViews
}

// GetBlockBytHeight 根据高度获取block
func (node *BraftNode) GetBlockBytHeight(height int64) *BlockView {
	views := node.GetBlocks(height, height+1)
	if len(views) == 0 {
		return nil
	}
	return views[0]
}

// GetBlockByID 获取BLock
func (node *BraftNode) GetBlockByID(id string) (*BlockView, error) {
	idreal, err := hex.DecodeString(id)
	if err != nil {
		return nil, err
	}
	block := node.blockStore.GetBlockByID(idreal)
	var preBlock *pb.BlockPack
	if block.PrevBlockId() != nil && len(block.PrevBlockId().Data) > 0 {
		preBlock = node.blockStore.GetBlockByID(block.PrevBlockId().Data)
	}
	var createdUsed int64
	if preBlock != nil {
		createdUsed = block.TimestampMs()/1000 - preBlock.TimestampMs()/1000
	}
	blockView := node.createBlockView(createdUsed, block)
	return blockView, nil
}

// GetTransacitonByTxID 根据tx_id查询 transaction 不同链的tx_id用相同的pre存储
func (node *BraftNode) GetTransacitonByTxID(txID string) *TxViewV1 {
	txQueryResult := node.txStore.GetTx(txID)
	if txQueryResult == nil {
		return nil
	}
	tx := txQueryResult.TxOld
	blockID := getHexString(txQueryResult.BlockID)
	txView := node.createTxView(blockID, txQueryResult.Height, tx)
	return txView
}

// FakeCommitBlock 人工写区块，仅做测试用
func (node *BraftNode) FakeCommitBlock(blockPack *pb.BlockPack) {
	node.blockStore.JustCommitIt(blockPack)
	node.txStore.OnNewBlockCommitted(blockPack)
}

// NodeView 节点数据
type NodeView struct {
	id        int
	IP        string `json:"ip"`
	HostName  string `json:"host_name"`
	IsLeader  bool   `json:"is_leader"`
	IsOnline  bool   `json:"is_online"`
	FiredCnt  int32  `json:"fired_cnt"` //被替换掉leader的次数
	EthHeight int64  `json:"eth_height"`
	BchHeight int64  `json:"bch_height"`
	BtcHeight int64  `json:"btc_height"`
}

// GetNodes 获取所有节点
func (node *BraftNode) GetNodes() []NodeView {
	nodeViews := make([]NodeView, 0)
	leaderID := cluster.LeaderNodeOfTerm(node.leader.term)
	apiLog.Error("leader id is", "leaderID", leaderID, "term is ", node.leader.term)
	nodeRuntimeInfos := node.peerManager.GetNodeRuntimeInfos()
	for _, node := range cluster.NodeList {
		var isLeader bool
		ip, _ := getHostAndPort(node.Url)
		if leaderID == node.Id {
			isLeader = true
		}
		var firedCnt int32
		ethH, btcH, bchH, LeaderCnt := getHeightAndLeaderCnt(node.Id, nodeRuntimeInfos)
		if isLeader && LeaderCnt > 0 {
			firedCnt = LeaderCnt - 1
		}
		nodeView := NodeView{
			IP:        ip,
			HostName:  node.Name,
			IsLeader:  isLeader,
			IsOnline:  node.IsNormal,
			EthHeight: ethH,
			BtcHeight: btcH,
			BchHeight: bchH,
			FiredCnt:  firedCnt,
		}
		nodeViews = append(nodeViews, nodeView)
	}
	sort.Slice(nodeViews, func(i, j int) bool {
		return nodeViews[i].id < nodeViews[j].id
	})
	return nodeViews
}

type ChainRegInfo struct {
	NewChain    string `json:"new_chain"`
	TargetChain string `json:"target_chain"`
}

type ChainRegID struct {
	ChainID uint32 `json:"chain_id"`
}

type TokenRegInfo struct {
	ContractAddr string `json:"contract_addr"`
	Chain        string `json:"chain"`
	ReceptChain  int32  `json:"recept_chain"`
	ReceptToken  int32  `json:"recept_token"`
}

type TokenRegID struct {
	TokenID uint32 `json:"token_id"`
}

// ChainRegister 新链注册
func (node *BraftNode) ChainRegister(regInfo *ChainRegInfo) {
	if regInfo.TargetChain == "eth" {
		proposal := strings.Join([]string{"CR", regInfo.TargetChain, regInfo.NewChain}, "_")
		_, err := node.ethWatcher.GatewayTransaction(node.signer.PubKeyHex, node.signer.PubkeyHash,
			ew.VOTE_METHOD_ADDCHAIN, regInfo.NewChain, proposal)
		if err != nil {
			nodeLogger.Error("register new chain failed", "err", err, "newchain", regInfo.NewChain)
		} else {
			nodeLogger.Debug("register new chain success", "newchain", regInfo.NewChain)
		}
	}
}

// GetChainRegisterID 查询链注册结果，如果成功，返回chainID，否则返回0
// func (node *BraftNode) GetChainRegisterID(newChain string, targetChain string) *ChainRegID {
// 	if targetChain == "eth" {
// 		chainID := node.ethWatcher.GetChainCode(targetChain)
// 		return &ChainRegID{ChainID: chainID}
// 	}
// 	return nil
// }

// TokenRegister 新token合约注册
func (node *BraftNode) TokenRegister(regInfo *TokenRegInfo) {
	if regInfo.Chain == "eth" {
		// 调用ETH合约接口注册新的token合约, proposal就使用contractaddr
		_, err := node.ethWatcher.GatewayTransaction(node.signer.PubKeyHex, node.signer.PubkeyHash, ew.VOTE_METHOD_ADDAPP,
			regInfo.ContractAddr, regInfo.ReceptChain, regInfo.ReceptToken, "TR_"+regInfo.ContractAddr)
		if err != nil {
			nodeLogger.Error("register new token failed", "err", err, "contractaddr", regInfo.ContractAddr)
		} else {
			nodeLogger.Debug("register new token success", "contractaddr", regInfo.ContractAddr)
		}
	}
}

// GetTokenRegisterID 查询token注册结果，如果成功，则返回tokenID, 否则返回0
func (node *BraftNode) GetTokenRegisterID(chain string, contractAddr string) *TokenRegID {
	if chain == "eth" {
		tokenID := node.ethWatcher.GetAppCode(contractAddr)
		return &TokenRegID{TokenID: tokenID}
	}
	return nil
}

func getHostAndPort(url string) (host, port string) {
	if url == "" {
		return
	}
	strs := strings.Split(url, ":")
	if len(strs) >= 2 {
		host = strs[0]
		port = strs[1]
	}
	return
}

func getHeightAndLeaderCnt(nodeID int32,
	apiDatas map[int32]*pb.NodeRuntimeInfo) (ethHeight, btcHeight, bchHeight int64, leaderCnt int32) {
	apiData := apiDatas[nodeID]
	if apiData == nil {
		return
	}
	return apiData.EthHeight, apiData.BtcHeight, apiData.BchHeight, apiData.LeaderCnt
}
