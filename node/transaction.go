package node

import (
	"bytes"
	"encoding/hex"
	"errors"
	btwatcher "swap/btwatcher"
	ew "swap/ethwatcher"
	"time"

	"github.com/antimoth/addrutils"
	"github.com/btcsuite/btcd/wire"
	"github.com/ofgp/common/defines"
	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/message"
	"github.com/ofgp/ofgp-core/primitives"
	pb "github.com/ofgp/ofgp-core/proto"
)

//transaction 相关

// txOperator 交易相关操作
type txOperator interface {
	CreateTx(req message.CreateReq) (*pb.NewlyTx, error)
	SendTx(req ISendReq) error
}

type AddrInfo struct {
	Addr   string
	Amount uint64
}

// BaseCreateReq createTx
type BaseCreateReq struct {
	Business uint32
	Chain    uint32
	ID       string
	Addr     []byte
	Amount   uint64
	From     uint8 //require tx所在链
}

// GetBusiness 获取所属业务
func (req *BaseCreateReq) GetBusiness() uint32 {
	return req.Business
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

// GetFrom 获取from chain
func (req *BaseCreateReq) GetFrom() uint8 {
	return req.From
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
	GetID() string
	GetTx() interface{}
}

// SendReq sendTx
type SendReq struct {
	Chain uint32
	ID    string
	Tx    interface{}
}

// GetChain 获取所在链
func (req *SendReq) GetChain() uint32 {
	return req.Chain
}

// GetID 交易标识id
func (req *SendReq) GetID() string {
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

func (eop *ethOperator) CreateTx(req message.CreateReq) (*pb.NewlyTx, error) {
	if ereq, ok := req.(*EthCreateReq); ok {
		nodeLogger.Debug("create eth tx", "scTxID", req.GetID())
		addrStr, err := addrutils.CheckBytesToStr(ereq.GetAddr(), uint8(req.GetChain()))
		if err != nil {
			leaderLogger.Error("addr to str err", "err", err, "scTxID", req.GetID())
			return nil, err
		}
		addr := ew.HexToAddress(addrStr)
		nodeLogger.Debug("create eth tx param", "method", ereq.Method, "appCode", ereq.TokenTo, "addr", addrStr, "amount", ereq.GetAmount(), "proposal", req.GetID(), "from", ereq.GetFrom())
		input, err := eop.cli.EncodeInput(ereq.Method, ereq.TokenTo, ereq.GetAmount(), addr, req.GetID(), ereq.GetFrom())
		if err != nil {
			leaderLogger.Error("create eth input failed", "err", err, "sctxid", req.GetID())
			return nil, err
		}
		return &pb.NewlyTx{Data: input}, nil
	}
	nodeLogger.Error("eth createReq type err", "id", req.GetID())
	return nil, errors.New("eth createReq type err")
}

// SendTx eth 无需send在sign阶段已经完成
func (eop *ethOperator) SendTx(req ISendReq) error {
	// tx := req.GetTx()
	// input, ok := tx.([]byte)
	// leaderLogger.Debug("send eth tx", "scTxID", req.GetID())
	// if !ok {
	// 	nodeLogger.Error("send eth req type err", "scTxID", req.GetID())
	// 	return errors.New("send eth req err")
	// }
	// _, err := eop.cli.SendTranxByInput(eop.signer.PubKeyHex, eop.signer.PubkeyHash, input)
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

func fromHex(s string) []byte {
	if len(s) > 1 {
		if s[0:2] == "0x" || s[0:2] == "0X" {
			s = s[2:]
		}
	}
	if len(s)%2 == 1 {
		s = "0" + s
	}
	return hex2Bytes(s)
}

func hex2Bytes(str string) []byte {
	h, _ := hex.DecodeString(str)

	return h
}

func sendCointTx(watcher *btwatcher.Watcher, req ISendReq, chain string) error {
	start := time.Now().UnixNano()
	tx := req.GetTx()
	var err error
	var txHash string
	if newlyTx, ok := tx.(*wire.MsgTx); ok {
		txHash, err = watcher.SendTx(newlyTx)
		end := time.Now().UnixNano()
		leaderLogger.Debug("sendCointime", "time", (end-start)/1e6, "chian", chain)
		if err != nil {
			leaderLogger.Error("send signed tx  failed", "err", err, "sctxid", req.GetID(), "chian", chain)
		}
		leaderLogger.Debug("sendTx", "scTxID", req.GetID(), "newTxHash", txHash)
	} else {
		leaderLogger.Debug("tx type err", "scTxID", req.GetID())
	}
	return err
}

// btcCreater 创建btc交易
type btcOprator struct {
	btcWatcher *btwatcher.Watcher
}

func newBtcOprator(watcher *btwatcher.Watcher) *btcOprator {
	return &btcOprator{
		btcWatcher: watcher,
	}
}

func (btcOP *btcOprator) CreateTx(req message.CreateReq) (*pb.NewlyTx, error) {
	nodeLogger.Debug("create btc tx", "scTxID", req.GetID())
	btTx, errCode := btcOP.btcWatcher.CreateCoinTx(req.GetAddr(), req.GetAmount(), cluster.ClusterSize,
		req.GetID(), req.GetFrom(), req.GetBusiness())
	if errCode != 0 {
		nodeLogger.Error("create tx fail", "scTxID", req.GetID(), "errCode", errCode)
		return nil, errors.New("create tx err")
	}
	buf := &bytes.Buffer{}
	err := btTx.Serialize(buf)
	if err != nil {
		nodeLogger.Error("serialize err", "err", err, "scTxID", req.GetID())
	}
	newTx := &pb.NewlyTx{
		Data: buf.Bytes(),
	}
	return newTx, nil
}

func (btcOP *btcOprator) SendTx(req ISendReq) error {
	leaderLogger.Debug("send btc tx", "scTxID", req.GetID())
	err := sendCointTx(btcOP.btcWatcher, req, "btc")
	return err
}

type bchOprator struct {
	bchWatcher *btwatcher.Watcher
}

func newBchOprator(watcher *btwatcher.Watcher) *bchOprator {
	return &bchOprator{
		bchWatcher: watcher,
	}
}

func (bchOP *bchOprator) CreateTx(req message.CreateReq) (*pb.NewlyTx, error) {
	addr := hex.EncodeToString(req.GetAddr())
	nodeLogger.Debug("create bch tx", "scTxID", req.GetID(), "adrr", addr, "amount", req.GetAmount(), "clusterSize", cluster.ClusterSize)
	btTx, errCode := bchOP.bchWatcher.CreateCoinTx(req.GetAddr(), req.GetAmount(), cluster.ClusterSize,
		req.GetID(), req.GetFrom(), req.GetBusiness())
	if errCode != 0 {
		nodeLogger.Error("create tx fail", "scTxID", req.GetID(), "errCode", errCode)
		return nil, errors.New("create tx err")
	}
	buf := &bytes.Buffer{}
	err := btTx.Serialize(buf)
	if err != nil {
		nodeLogger.Error("deserialize err", "err", err, "scTxID", req.GetID())
	}
	newTx := &pb.NewlyTx{
		Data: buf.Bytes(),
	}
	return newTx, nil
}

func (bchOP *bchOprator) SendTx(req ISendReq) error {
	leaderLogger.Debug("send bch tx", "scTxID", req.GetID())
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

func (ti *txInvoker) CreateTx(req message.CreateReq) (*pb.NewlyTx, error) {
	var newTx *pb.NewlyTx
	var err error
	chain := uint8(req.GetChain())
	switch chain {
	case defines.CHAIN_CODE_BCH:
		newTx, err = ti.bchOp.CreateTx(req)
	case defines.CHAIN_CODE_ETH:
		newTx, err = ti.ethOp.CreateTx(req)
	case defines.CHAIN_CODE_BTC:
		newTx, err = ti.btcOp.CreateTx(req)
	default:
		nodeLogger.Error("create req chain type err", "chain", chain)
	}
	return newTx, err
}

func (ti *txInvoker) SendTx(req ISendReq) error {
	var err error
	chain := uint8(req.GetChain())
	switch chain {
	case defines.CHAIN_CODE_BCH:
		err = ti.bchOp.SendTx(req)
	case defines.CHAIN_CODE_ETH:
		err = ti.ethOp.SendTx(req)
	case defines.CHAIN_CODE_BTC:
		err = ti.btcOp.SendTx(req)
	default:
		return errors.New("not found")
	}
	return err
}
