package node

import (
	"bytes"
	"time"

	"github.com/btcsuite/btcd/wire"
	btcwatcher "github.com/ofgp/bitcoinWatcher/mortgagewatcher"
	ew "github.com/ofgp/ethwatcher"
	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/primitives"
	pb "github.com/ofgp/ofgp-core/proto"
)

//transaction 相关

type txOperator interface {
	CreateTx() *pb.NewlyTx
	SendTx(req ISendReq) error
}

// ethCreater 创建eth交易
type ethOperator struct {
	cli    *ew.Client
	bs     *primitives.BlockStore
	signer *crypto.SecureSigner
}
type AddrInfo struct {
	Addr   string
	Amount uint64
}
type CreateReq interface {
	GetID() string
	GetFee() uint64
	GetAddrInfos() []AddrInfo
}

type BaseCreateReq struct {
	ID        string
	Fee       uint64
	AddrInfos []AddrInfo
}

type EthCreateReq struct {
	BaseCreateReq
	Method  string
	TokenTo uint32
}

func (req *BaseCreateReq) GetFee() uint64 {
	return req.Fee
}
func (req *BaseCreateReq) GetID() string {
	return req.ID
}
func (req *BaseCreateReq) GetAddrInfos() []AddrInfo {
	return req.AddrInfos
}

type ISendReq interface {
	GetID() string
	GetTx() *pb.NewlyTx
}
type SendReq struct {
	ID string
	Tx *pb.NewlyTx
}

func (eop *ethOperator) CreateTx(req CreateReq) *pb.NewlyTx {
	if ereq, ok := req.(*EthCreateReq); ok {
		addrInfo := ereq.GetAddrInfos()[0]
		addredss := ew.HexToAddress(addrInfo.Addr)
		input, err := eop.cli.EncodeInput(ew.VOTE_METHOD_MINT, ereq.TokenTo, addrInfo.Amount,
			addredss, req.GetID())
		if err != nil {
			leaderLogger.Error("create eth input failed", "err", err, "sctxid", req.GetID())
			return nil
		}
		return &pb.NewlyTx{Data: input}
	} else {
		nodeLogger.Error("req err", "id", req.GetID())
	}
	return nil
}

func (eop *ethOperator) SendTx(req ISendReq) error {
	_, err := eop.cli.SendTranxByInput(eop.signer.PubKeyHex, eop.signer.PubkeyHash, req.GetTx().Data)
	if err != nil {
		nodeLogger.Error("send eth tx err", "error", err, "id", req.GetID())
	}
	return err
}

// BtcCreateReq 创建btc交易请求
type BtcCreateReq struct {
	BaseCreateReq
}

// BchCreateReq 创建bch交易请求
type BchCreateReq struct {
	BaseCreateReq
}

// createCoinTx 创建bch/btc tx
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
	buf := bytes.NewBuffer(req.GetTx().Data)
	newlyTx := new(wire.MsgTx)
	err := newlyTx.Deserialize(buf)
	start := time.Now().UnixNano()
	_, err = watcher.SendTx(newlyTx)
	end := time.Now().UnixNano()
	leaderLogger.Debug("sendCointime", "time", (end-start)/1e6, "chian", chain)
	if err != nil {
		leaderLogger.Error("send signed tx  failed", "err", err, "sctxid", req.GetID(), "chian", chain)
	}
	return err
}

// btcCreater 创建btc交易
type btcOprator struct {
	btcWatcher *btcwatcher.MortgageWatcher
}

func (btcOP *btcOprator) CreateTx(req CreateReq) *pb.NewlyTx {
	return createCoinTx(btcOP.btcWatcher, nil, req.GetFee(), req.GetID())
}

func (btcOP *btcOprator) SendTx(req ISendReq) error {
	err := sendCointTx(btcOP.btcWatcher, req, "btc")
	return err
}

type bchOprator struct {
	bchWatcher *btcwatcher.MortgageWatcher
}

func (bchOP *bchOprator) CreateTx(req CreateReq) *pb.NewlyTx {
	return createCoinTx(bchOP.bchWatcher, nil, req.GetFee(), req.GetID())
}

func (bchOP *bchOprator) SendTx(req ISendReq) error {
	err := sendCointTx(bchOP.bchWatcher, req, "bch")
	return err
}
