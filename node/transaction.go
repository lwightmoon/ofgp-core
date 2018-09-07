package node

import (
	"bytes"
	"time"

	"github.com/btcsuite/btcd/wire"
	btcwatcher "github.com/ofgp/bitcoinWatcher/mortgagewatcher"
	ew "github.com/ofgp/ethwatcher"
	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/message"
	"github.com/ofgp/ofgp-core/primitives"
	pb "github.com/ofgp/ofgp-core/proto"
)

//transaction 相关

// txOperator 交易相关操作
type txOperator interface {
	CreateTx(req CreateReq) (*pb.NewlyTx, error)
	SendTx(req ISendReq) error
}

type AddrInfo struct {
	Addr   string
	Amount uint64
}

// CreateReq 创建交易接口
type CreateReq interface {
	GetChain() uint32
	GetID() string
	GetAddr() []byte
	GetAmount() uint64
}

// BaseCreateReq createTx
type BaseCreateReq struct {
	Chain  uint32
	ID     string
	Addr   []byte
	Amount uint64
}

// GetChain 创建交易类型
func (req *BaseCreateReq) GetChain() uint32 {
	return req.Chain
}

// GetID 创建交易标识id
func (req *BaseCreateReq) GetID() string {
	return req.ID
}

// GetAddr 获取发送到的地址
func (req *BaseCreateReq) GetAddr() []byte {
	return req.Addr
}

// GetAmount 获取发送金额
func (req *BaseCreateReq) GetAmount() uint64 {
	return req.Amount
}

// EthCreateReq eth createTx
type EthCreateReq struct {
	BaseCreateReq
	Method  string
	TokenTo uint32
}

// ISendReq sendTx
type ISendReq interface {
	GetChain() uint32
	GetID() []byte
	GetTx() interface{}
}

// SendReq sendTx
type SendReq struct {
	Chain uint32
	ID    []byte
	Tx    interface{}
}

// GetChain 获取所在链
func (req *SendReq) GetChain() uint32 {
	return req.Chain
}

// GetID 交易标识id
func (req *SendReq) GetID() []byte {
	return req.ID
}

// GetTx 获取交易[]byte
func (req *SendReq) GetTx() interface{} {
	return req.Tx
}

// ethCreater 创建eth交易
type ethOperator struct {
	cli    *ew.Client
	bs     *primitives.BlockStore
	signer *crypto.SecureSigner
}

func newEthOperator(cli *ew.Client, bs *primitives.BlockStore, signer *crypto.SecureSigner) *ethOperator {
	return &ethOperator{
		cli,
		bs,
		signer,
	}
}

func (eop *ethOperator) CreateTx(req CreateReq) (*pb.NewlyTx, error) {
	if ereq, ok := req.(*EthCreateReq); ok {
		// addrInfo := ereq.GetAddrInfos()[0]
		// addredss := ew.HexToAddress(addrInfo.Addr)
		// input, err := eop.cli.EncodeInput(ew.VOTE_METHOD_MINT, ereq.TokenTo, addrInfo.Amount,
		// 	addredss, req.GetID())
		// if err != nil {
		// 	leaderLogger.Error("create eth input failed", "err", err, "sctxid", req.GetID())
		// 	return nil
		// }
		// return &pb.NewlyTx{Data: input}
		nodeLogger.Debug("eth create req:%v", ereq)
	} else {
		nodeLogger.Error("req err", "id", req.GetID())
	}
	return nil, nil
}

func (eop *ethOperator) SendTx(req ISendReq) error {
	// _, err := eop.cli.SendTranxByInput(eop.signer.PubKeyHex, eop.signer.PubkeyHash, req.GetTx().Data)
	// if err != nil {
	// 	nodeLogger.Error("send eth tx err", "error", err, "id", req.GetID())
	// }
	// return err
	return nil
}

// BtcCreateReq 创建btc交易请求
type BtcCreateReq struct {
	BaseCreateReq
}

// BchCreateReq 创建bch交易请求
type BchCreateReq struct {
	BaseCreateReq
}

// createCoinTx 创建bch/btc tx 废止！
func createCoinTx(watcher *btcwatcher.MortgageWatcher,
	addrInfos []*btcwatcher.AddressInfo, fee uint64, id string) *pb.NewlyTx {
	leaderLogger.Debug("rechargelist", "sctxid", id, "addrs", addrInfos)
	newlyTx, ok := watcher.CreateCoinTx(addrInfos, int64(fee), id)
	if ok != 0 {
		leaderLogger.Error("create new chan tx failed", "errcode", ok, "sctxid", id)
		return nil
	}
	leaderLogger.Debug("create coin tx", "sctxid", id, "newlyTxid", newlyTx.TxHash().String())

	buf := bytes.NewBuffer([]byte{})
	err := newlyTx.Serialize(buf)
	if err != nil {
		leaderLogger.Error("serialize newly tx failed", "err", err)
		return nil
	}
	return &pb.NewlyTx{Data: buf.Bytes()}
}

func sendCointTx(watcher *btcwatcher.MortgageWatcher, req ISendReq, chain string) error {
	// buf := bytes.NewBuffer(req.GetTx().Data)
	// newlyTx := new(wire.MsgTx)
	// err := newlyTx.Deserialize(buf)
	start := time.Now().UnixNano()
	tx := req.GetTx()
	var err error
	if newlyTx, ok := tx.(*wire.MsgTx); ok {
		_, err = watcher.SendTx(newlyTx)
		end := time.Now().UnixNano()
		leaderLogger.Debug("sendCointime", "time", (end-start)/1e6, "chian", chain)
		if err != nil {
			leaderLogger.Error("send signed tx  failed", "err", err, "sctxid", req.GetID(), "chian", chain)
		}
	}

	return err
}

// btcCreater 创建btc交易
type btcOprator struct {
	btcWatcher *btcwatcher.MortgageWatcher
}

func newBtcOprator(watcher *btcwatcher.MortgageWatcher) *btcOprator {
	return &btcOprator{
		btcWatcher: watcher,
	}
}

func (btcOP *btcOprator) CreateTx(req CreateReq) (*pb.NewlyTx, error) {
	return nil, nil
}

func (btcOP *btcOprator) SendTx(req ISendReq) error {
	err := sendCointTx(btcOP.btcWatcher, req, "btc")
	return err
}

type bchOprator struct {
	bchWatcher *btcwatcher.MortgageWatcher
}

func newBchOprator(watcher *btcwatcher.MortgageWatcher) *bchOprator {
	return &bchOprator{
		bchWatcher: watcher,
	}
}

func getBtcAddrInfos(addrInfos []AddrInfo) []*btcwatcher.AddressInfo {
	res := make([]*btcwatcher.AddressInfo, 0)
	for _, addrInfo := range addrInfos {
		btcAddrInfo := &btcwatcher.AddressInfo{
			Address: addrInfo.Addr,
			Amount:  int64(addrInfo.Amount),
		}
		res = append(res, btcAddrInfo)
	}
	return res
}
func (bchOP *bchOprator) CreateTx(req CreateReq) (*pb.NewlyTx, error) {
	return nil, nil
}

func (bchOP *bchOprator) SendTx(req ISendReq) error {
	err := sendCointTx(bchOP.bchWatcher, req, "bch")
	return err
}

// txInvoker 命令执行
type txInvoker struct {
	ethOp *ethOperator
	bchOp *bchOprator
	btcOp *btcOprator
}

func newTxInvoker(ethOp *ethOperator, bchOp *bchOprator, btcOp *btcOprator) *txInvoker {
	return &txInvoker{
		ethOp: ethOp,
		bchOp: bchOp,
		btcOp: btcOp,
	}
}

func (ti *txInvoker) CreateTx(req CreateReq) (*pb.NewlyTx, error) {
	var newTx *pb.NewlyTx
	var err error
	switch req.GetChain() {
	case message.Bch:
		newTx, err = ti.bchOp.CreateTx(req)
	case message.Eth:
		newTx, err = ti.ethOp.CreateTx(req)
	case message.Btc:
		newTx, err = ti.btcOp.CreateTx(req)
	}
	return newTx, err
}

func (ti *txInvoker) SendTx(req ISendReq) error {
	var err error
	switch req.GetChain() {
	case message.Bch:
		err = ti.bchOp.SendTx(req)
	case message.Eth:
		err = ti.ethOp.SendTx(req)
	case message.Btc:
		err = ti.btcOp.SendTx(req)
	}
	return err
}
