package node

type PushEvent interface {
	GetBusiness() string
	GetMethod() string
	GetFrom() string
	GetTo() string
	GetTxID() string
	GetData() []byte
}

type BusinessEvent interface {
	GetData() PushEvent
	GetErr() error
}

type WatchedEvent struct {
	Data PushEvent
	Err  error
}

type SignedEvent struct {
	Chain string
	ID    string //业务id
	TxID  string
	Data  []byte //签名后数据
}
