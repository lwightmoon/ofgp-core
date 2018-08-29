package p2p

import (
	"fmt"
)

// getExchangeInfo 获取交换数据
func (info *P2PInfo) getExchangeInfo() (chain uint32, addr string, amount uint64) {
	chain = info.Msg.GetChain()
	addr = info.Msg.GetRequireAddr()
	amount = info.Msg.GetAmount()
	return
}
func (info *P2PInfo) GetScTxID() string {
	event := info.Event
	if event != nil {
		return event.GetTxID()
	}
	p2pLogger.Error("event nil p2pInfo")
	return ""
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

func (ti *txIndex) Add(info *P2PInfo) {
	chain := fmt.Sprintf("%d", info.Event.From)
	child := ti.root.add(chain)
	sendAddr := info.Msg.SendAddr
	child = child.add(sendAddr)
	amount := fmt.Sprintf("%d", info.Msg.Amount)
	child = child.add(amount)
	txID := info.Event.TxID
	p2pLogger.Debug("index", "txID", txID)
	child.add(txID)
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
func (tn *TxNode) getNodeVals() []string {
	var vals []string
	for _, node := range tn.Childs {
		vals = append(vals, node.Value)
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
	txIDNode := amountNode.getNode(amountStr)
	if txIDNode == nil {
		return nil
	}
	txIDs := txIDNode.getNodeVals()
	return txIDs
}
