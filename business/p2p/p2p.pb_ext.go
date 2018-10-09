package p2p

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/antimoth/swaputils"
)

// getExchangeInfo 获取交换数据
func (info *P2PInfo) getExchangeInfo() (chain uint32, addr string, amount uint64) {
	chain = info.Msg.GetChain()
	var err error
	addr, err = swaputils.CheckBytesToStr(info.Msg.GetRequireAddr(), uint8(chain))
	if err != nil {
		p2pLogger.Error("addr to str err", "err", err)
		return
	}
	amount = info.Msg.GetAmount()
	return
}

// GetScTxID 获取监听到交易的txid
func (info *P2PInfo) GetScTxID() string {
	event := info.Event
	if event != nil {
		return event.GetTxID()
	}
	p2pLogger.Error("event nil p2pInfo")
	return ""
}

// IsExpired 是否匹配交易超时
func (info *P2PInfo) IsExpired() bool {
	return uint32(time.Now().Unix()) > info.Msg.ExpiredTime
}

// isConfirmTimeout 是否超时未被确认
func (info *P2PInfo) isConfirmTimeout(timeout int64) bool {
	if time.Now().Unix()-info.Time > timeout {
		return true
	}
	return false
}
func (info *P2PInfo) GetExchangeTxParam() {

}
func (info *P2PInfo) GetBackTxParam() {
}

func (info *P2PConfirmInfo) GetTxID() string {
	event := info.Event
	if event != nil {
		return event.GetTxID()
	}
	p2pLogger.Error("event nil p2pConfirmInfo")
	return ""
}

type txIndex struct {
	root *TxNode
}

func newTxIndex() *txIndex {
	return &txIndex{
		root: &TxNode{
			Value:  "root",
			Childs: make(map[string]*TxNode),
		},
	}
}
func (ti *txIndex) AddInfos(infos []*P2PInfo) {
	for _, info := range infos {
		ti.Add(info)
	}
}
func (ti *txIndex) Add(info *P2PInfo) {
	chain := fmt.Sprintf("%d", info.Event.From)
	child := ti.root.add(chain)
	sendAddr, err := swaputils.CheckBytesToStr(info.Msg.GetSendAddr(), uint8(info.Event.From))
	if err != nil {
		p2pLogger.Error("send addr to str err", "err", err)
		return
	}
	child = child.add(sendAddr)
	amount := fmt.Sprintf("%d", info.Event.Amount)
	child = child.add(amount)
	index := fmt.Sprintf("%d", info.GetIndex())
	child = child.add(index)
	txID := info.Event.TxID
	child.add(txID)
}

func (ti *txIndex) Del(info *P2PInfo) {
	chain := fmt.Sprintf("%d", info.Event.From)
	sendAddr, err := swaputils.CheckBytesToStr(info.Msg.GetSendAddr(), uint8(info.Event.From))
	if err != nil {
		p2pLogger.Error("send addr to str err", "err", err)
		return
	}
	amount := fmt.Sprintf("%d", info.Event.Amount)
	index := fmt.Sprintf("%d", info.Index)
	addrNode := ti.root.getNode(chain)
	txID := info.GetScTxID()
	if addrNode == nil {
		return
	}
	amountNode := addrNode.getNode(sendAddr)
	if amountNode == nil {
		return
	}
	txIndexNode := amountNode.getNode(amount)
	if txIndexNode == nil {
		return
	}
	txIDNode := txIndexNode.getNode(index)
	if txIDNode == nil {
		return
	}

	delete(txIDNode.Childs, txID)
	if len(txIDNode.Childs) == 0 {
		delete(txIndexNode.Childs, index)
	}
	if len(txIndexNode.Childs) == 0 {
		delete(amountNode.Childs, amount)
	}
	if len(amountNode.Childs) == 0 {
		delete(addrNode.Childs, sendAddr)
	}
	if len(addrNode.Childs) == 0 {
		delete(ti.root.Childs, chain)
	}
}
func (tn *TxNode) add(val string) *TxNode {
	if node, ok := tn.Childs[val]; ok {
		return node
	}
	node := &TxNode{
		Value:  val,
		Childs: make(map[string]*TxNode),
	}
	tn.Childs[val] = node
	return node
}
func (tn *TxNode) getNode(key string) *TxNode {
	return tn.Childs[key]
}
func (tn *TxNode) getNodeVals() []int {
	var vals []int
	for _, node := range tn.Childs {
		if node.Value != "" {
			num, _ := strconv.Atoi(node.Value)
			vals = append(vals, num)
		}
	}
	return vals
}

// GetTxID 根据所在链 地址 数量 查找txID
func (ti *txIndex) GetTxID(chain uint32, addr string, amount uint64) []string {
	chainStr := fmt.Sprintf("%d", chain)
	addrNode := ti.root.getNode(chainStr)
	if addrNode == nil {
		return nil
	}
	amountNode := addrNode.getNode(addr)
	if amountNode == nil {
		return nil
	}
	amountStr := fmt.Sprintf("%d", amount)
	txIndexNode := amountNode.getNode(amountStr)
	if txIndexNode == nil {
		return nil
	}
	indexs := txIndexNode.getNodeVals()
	sort.Ints(indexs)
	fmt.Printf("---------indexs:%v\n", indexs)
	var txIDs []string
	for _, index := range indexs {
		key := strconv.Itoa(index)
		txIDNode := txIndexNode.Childs[key]
		fmt.Println(txIDNode)
		for txID := range txIDNode.Childs {
			txIDs = append(txIDs, txID)
		}
	}
	return txIDs
}
