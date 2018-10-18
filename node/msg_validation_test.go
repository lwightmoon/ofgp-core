package node

import (
	"testing"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/crypto"
	pb "github.com/ofgp/ofgp-core/proto"
)

var signer *crypto.SecureSigner

func init() {
	url := "http://47.97.167.221:8976"
	signer = crypto.NewSecureSigner("04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787", "3722834BCB13F7308C28907B69A99DB462F39036")
	signer.InitKeystoreParam("C51C9CB7A7EC9D12BB37B3700856690719A44056B750AB03A21247A4903BF3CB", "0daf7126-ebbb-4b2d-86f8-a480c1fd45a8", url)
	cluster.NodeList = []cluster.NodeInfo{
		cluster.NodeInfo{
			PublicKey: signer.Pubkey,
		},
	}
}
func TestValidateSignReq(t *testing.T) {
	event := &pb.WatchedEvent{
		Business:  "p2p",
		EventType: 1,
		TxID:      "testTxID",
		From:      1,
		To:        1,
		Data:      []byte("test"),
	}
	newTx := &pb.NewlyTx{}
	req, err := pb.MakeSignReqMsg(0, 0, event, newTx, "", signer, nil, 1)
	if err != nil {
		t.Errorf("create signreq err:%v", err)
	}
	passed := checkSignRequest(req)
	if !passed {
		t.Error("checkSignreq fail")
	}
}

func TestValidateSignRes(t *testing.T) {
	var data = [][]byte{
		[]byte("test"),
	}
	signRes, err := pb.MakeSignResult("p2p", 1, 0, "txID_test", data, 2, 0, signer)
	if err != nil {
		t.Errorf("create signres fail")
	}
	passed := checkSignedResult(signRes)
	if !passed {
		t.Error("checkSignreq fail")
	}
}
