package p2p

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/node"
	"github.com/spf13/viper"
)

func initCuster(tmpDir string) {
	viper.Set("KEYSTORE.count", 2)
	viper.Set("KEYSTORE.keystore_private_key", "C51C9CB7A7EC9D12BB37B3700856690719A44056B750AB03A21247A4903BF3CB")
	viper.Set("KEYSTORE.service_id", "0daf7126-ebbb-4b2d-86f8-a480c1fd45a8")
	viper.Set("KEYSTORE.url", "http://47.98.185.203:8976")
	viper.Set("KEYSTORE.redeem", "524104a2e82be35d90d954e15cc5865e2f8ac22fd2ddbd4750f4bfc7596363a3451d1b75f4a8bad28cf48f63595349dbc141d6d6e21f4feb65bdc5e1a8382a2775e78741049fd6230e3badbbc7ba190e10b2fc5c3d8ea9b758a43e98ab2c8f83c826ae7eabea6d88880bc606fa595cd8dd17fc7784b3e55d8ee0705045119545a803215b8041044667e5b36f387c4d8d955c33fc271f46d791fd3433c0b2f517375bbd9aae6b8c2392229537b109ac8eadcce104aeaa64db2d90bef9008a09f8563cdb05ffb60b53ae")
	viper.Set("KEYSTORE.deferation_address", "bchreg:pzzx7c5l0zlxyde36gwkhm7ua86qlvk90y2ecnynug")

	viper.Set("KEYSTORE.hash_0", "3722834BCB13F7308C28907B69A99DB462F39036")
	viper.Set("KEYSTORE.key_0", "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787")
	viper.Set("KEYSTORE.hash_1", "E37B5BEBF46B6CAA4B2146CCD83D61966B33687A")
	viper.Set("KEYSTORE.key_1", "049FD6230E3BADBBC7BA190E10B2FC5C3D8EA9B758A43E98AB2C8F83C826AE7EABEA6D88880BC606FA595CD8DD17FC7784B3E55D8EE0705045119545A803215B80")

	viper.Set("KEYSTORE.local_pubkey_hash", "3722834BCB13F7308C28907B69A99DB462F39036")
	viper.Set("DGW.dbpath", tmpDir)
	viper.Set("DGW.count", 2)
	viper.Set("DGW.local_id", 0)
	viper.Set("DGW.bch_height", 100)
	viper.Set("DGW.eth_height", 100)
	viper.Set("DGW.local_p2p_port", 10000)
	viper.Set("DGW.local_http_port", 8080)

	viper.Set("DGW.host_0", "127.0.0.1:10000")
	viper.Set("DGW.status_0", true)
	viper.Set("DGW.host_1", "127.0.0.1:10001")
	viper.Set("DGW.status_1", true)

	viper.Set("DGW.eth_confirm_count", 6)
	viper.Set("DGW.eth_client_url", "ws://47.98.185.203:8830")
	viper.Set("coin_type", "bch")
	viper.Set("net_param", "regtest")
	viper.Set("BCH.rpc_server", "47.97.167.221:8445")
	viper.Set("BCH.rpc_user", "tanshaohua")
	viper.Set("BCH.rpc_password", "hahaha")
	viper.Set("BCH.confirm_block_num", 1)
	viper.Set("BCH.coinbase_confirm_block_num", 100)
	viper.Set("DGW.start_mode", 3)
	cluster.Init()
}

func TestMain(m *testing.M) {
	tmpDir, err := ioutil.TempDir("", "braft")
	if err != nil {
		panic("create tempdir failed")
	}
	initCuster(tmpDir)
	defer os.RemoveAll(tmpDir)
	code := m.Run()
	os.Exit(code)
}
func TestProcess(t *testing.T) {
	node := node.NewBraftNode(cluster.NodeInfo{
		Id:       0,
		Name:     "server0",
		Url:      "127.0.0.1:10000",
		IsNormal: true,
	})
	p2p := NewP2P(node)
	p2p.processEvent()
}
