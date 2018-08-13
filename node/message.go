package node

//watcher交互event
type PushEvent interface {
	GetBusiness() string
	GetMethod() uint8
	GetFrom() uint8
	GetTo() uint8
	GetData() []byte
}

// BusinessEvent 业务交互event
type BusinessEvent interface {
	GetBusiness() string
	GetData() interface{}
	GetErr() error
}

// WatchedEvent 交易被监听到
type WatchedEvent struct {
	business string
	data     PushEvent
	err      error
}

func (we *WatchedEvent) GetBusiness() string {
	return we.business
}
func (we *WatchedEvent) GetData() interface{} {
	return we.data
}
func (we *WatchedEvent) GetErr() error {
	return we.err
}

//SignedData 签名数据
type SignedData struct {
	Chain string
	ID    string //业务id
	TxID  string
	Data  []byte //签名后数据
}
type SignedEvent struct {
	business string
	data     SignedData
	err      error //1、节点正在同步数据 2、签名失败
}

func (se *SignedEvent) GetBusiness() string {
	return se.business
}
func (se *SignedEvent) GetData() interface{} {
	return se.data
}
func (se *SignedEvent) GetErr() error {
	return se.err
}

// SendedData 发送数据
type SendedData struct {
	Chain string //链
	TxID  string //签名后的id
}

// SendedEvent 已发送事件
type SendedEvent struct {
	business string
	data     SendedData
	err      error //发送交易失败
}

// 交易被确认
type ConfirmData struct {
	Chain    string //链
	TxID     string //最终被确认的交易id
	Confirms int    //确认数
}

// 交易已确认
type ConfirmEvent struct {
	business string
	data     ConfirmData
	err      error //等待交易确认超时
}

func (ce *ConfirmEvent) GetBusiness() string {
	return ce.business
}
func (ce *ConfirmEvent) GetData() interface{} {
	return ce.data
}
func (ce *ConfirmEvent) GetErr() error {
	return ce.err
}

type CommitedData struct {
	txID   string //网关交易id
	height int    //网关区块高度
	index  int    //在区块中的索引
}

// CommitedEvent 交易被提交
type CommitedEvent struct {
	business string
	data     CommitedData
	err      error
}

func (ce *CommitedEvent) GetBusiness() string {
	return ce.business
}
func (ce *CommitedEvent) GetData() interface{} {
	return ce.data
}
func (ce *CommitedEvent) GetErr() error {
	return ce.err
}
