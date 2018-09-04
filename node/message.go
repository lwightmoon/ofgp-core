package node

import (
	pb "github.com/ofgp/ofgp-core/proto"
)

//watcher交互event
type PushEvent interface {
	GetBusiness() string
	GetEventType() uint32 //0 初始监听到 1 被确认
	GetTxID() string
	GetAmount() uint64 //金额
	GetFee() uint64    //矿工费
	GetFrom() uint32
	GetTo() uint32
	GetData() []byte
}

// BusinessEvent 业务交互event
type BusinessEvent interface {
	GetBusiness() string
	GetErr() error
}

type BaseBusinessEvent struct {
	Business string
	Err      error
}

func (e *BaseBusinessEvent) GetBusiness() string {
	return e.Business
}
func (e *BaseBusinessEvent) GetErr() error {
	return e.Err
}

// WatchedEvent 交易被监听到
type WatchedEvent struct {
	BaseBusinessEvent
	Data *pb.WatchedEvent
}

func getWatchedEvent(event PushEvent) *pb.WatchedEvent {
	return &pb.WatchedEvent{
		Business:  event.GetBusiness(),
		EventType: event.GetEventType(),
		TxID:      event.GetTxID(),
		Amount:    uint32(event.GetAmount()),
		Fee:       uint32(event.GetFee()),
		From:      uint32(event.GetFrom()),
		To:        uint32(event.GetTo()),
		Data:      event.GetData(),
	}
}

func newWatchedEvent(event PushEvent) *WatchedEvent {
	res := &WatchedEvent{}
	res.Business = event.GetBusiness()
	res.Err = nil
	data := getWatchedEvent(event)
	res.Data = data
	return res
}

func (we *WatchedEvent) GetData() *pb.WatchedEvent {
	return we.Data
}

//SignedData 签名数据
type SignedData struct {
	Chain uint32
	ID    string //业务id
	TxID  string
	Term  int64
	Data  interface{} //签名后数据 跟具体的链相关
}
type SignedEvent struct {
	BaseBusinessEvent
	Data *SignedData
}

func newSignedEvent(business string, data *SignedData, err error) *SignedEvent {
	res := &SignedEvent{}
	res.Business = business
	res.Data = data
	res.Err = err
	return res
}

func (se *SignedEvent) GetData() *SignedData {
	return se.Data
}

// 交易已确认
type ConfirmEvent struct {
	BaseBusinessEvent
	Data *pb.WatchedEvent
}

func newConfirmEvent(event PushEvent) *ConfirmEvent {
	res := &ConfirmEvent{}
	res.Business = event.GetBusiness()
	data := getWatchedEvent(event)
	res.Data = data
	return res
}

func (ce *ConfirmEvent) GetData() *pb.WatchedEvent {
	return ce.Data
}

type CommitedData struct {
	Tx     *pb.Transaction
	Height int64 //网关区块高度
	Index  int   //在区块中的索引
}

// CommitedEvent 交易被提交
type CommitedEvent struct {
	BaseBusinessEvent
	Data *CommitedData
}

func newCommitedEvent(business string, data *CommitedData, err error) *CommitedEvent {
	res := &CommitedEvent{}
	res.Business = business
	res.Data = data
	res.Err = err
	return res
}

func (ce *CommitedEvent) GetData() *CommitedData {
	return ce.Data
}
