package node

import (
	ew "github.com/ofgp/ethwatcher"
	pb "github.com/ofgp/ofgp-core/proto"
)

//transaction 相关

type txCreater interface {
	CreateTx() *pb.NewlyTx
}

type ethCreater struct {
	cli *ew.Client
}

func (ec ethCreater) CreateTx() *pb.NewlyTx {
	return nil
}
