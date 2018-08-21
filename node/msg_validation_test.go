package node

import (
	"testing"

	"github.com/ofgp/ofgp-core/cluster"
)

func TestMain(t *testing.M) {
	cluster.NodeList = []cluster.NodeInfo{
		cluster.NodeInfo{
			PublicKey: []byte("t"),
		},
	}
}
func TestValidateSignReq(t *testing.T) {

}
