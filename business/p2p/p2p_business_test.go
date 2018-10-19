package p2p

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/config"
	"github.com/ofgp/ofgp-core/dgwdb"
	"github.com/ofgp/ofgp-core/node"
	pb "github.com/ofgp/ofgp-core/proto"
)

func initCuster(tmpDir string) {
	conf := &config.Config{}
	// viper.Set("coin_type", "bch")
	// viper.Set("net_param", "regtest")
	conf.NetParam = "regtest"
	// viper.Set("KEYSTORE.count", 2)
	// viper.Set("KEYSTORE.keystore_private_key", "C51C9CB7A7EC9D12BB37B3700856690719A44056B750AB03A21247A4903BF3CB")
	// viper.Set("KEYSTORE.service_id", "0daf7126-ebbb-4b2d-86f8-a480c1fd45a8")
	// viper.Set("KEYSTORE.url", "http://47.98.185.203:8976")
	// viper.Set("KEYSTORE.redeem", "524104a2e82be35d90d954e15cc5865e2f8ac22fd2ddbd4750f4bfc7596363a3451d1b75f4a8bad28cf48f63595349dbc141d6d6e21f4feb65bdc5e1a8382a2775e78741049fd6230e3badbbc7ba190e10b2fc5c3d8ea9b758a43e98ab2c8f83c826ae7eabea6d88880bc606fa595cd8dd17fc7784b3e55d8ee0705045119545a803215b8041044667e5b36f387c4d8d955c33fc271f46d791fd3433c0b2f517375bbd9aae6b8c2392229537b109ac8eadcce104aeaa64db2d90bef9008a09f8563cdb05ffb60b53ae")
	// viper.Set("KEYSTORE.deferation_address", "bchreg:pzzx7c5l0zlxyde36gwkhm7ua86qlvk90y2ecnynug")
	// viper.Set("KEYSTORE.hash_0", "3722834BCB13F7308C28907B69A99DB462F39036")
	// viper.Set("KEYSTORE.key_0", "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787")
	// viper.Set("KEYSTORE.hash_1", "E37B5BEBF46B6CAA4B2146CCD83D61966B33687A")
	// viper.Set("KEYSTORE.key_1", "049FD6230E3BADBBC7BA190E10B2FC5C3D8EA9B758A43E98AB2C8F83C826AE7EABEA6D88880BC606FA595CD8DD17FC7784B3E55D8EE0705045119545A803215B80")
	// viper.Set("KEYSTORE.local_pubkey_hash", "3722834BCB13F7308C28907B69A99DB462F39036")
	keyConf := config.KeyStore{
		URL:                "http://47.98.185.203:8976",
		Count:              2,
		KeyStorePrivateKey: "C51C9CB7A7EC9D12BB37B3700856690719A44056B750AB03A21247A4903BF3CB",
		ServiceID:          "0daf7126-ebbb-4b2d-86f8-a480c1fd45a8",
		Redeem:             "524104a2e82be35d90d954e15cc5865e2f8ac22fd2ddbd4750f4bfc7596363a3451d1b75f4a8bad28cf48f63595349dbc141d6d6e21f4feb65bdc5e1a8382a2775e78741049fd6230e3badbbc7ba190e10b2fc5c3d8ea9b758a43e98ab2c8f83c826ae7eabea6d88880bc606fa595cd8dd17fc7784b3e55d8ee0705045119545a803215b8041044667e5b36f387c4d8d955c33fc271f46d791fd3433c0b2f517375bbd9aae6b8c2392229537b109ac8eadcce104aeaa64db2d90bef9008a09f8563cdb05ffb60b53ae",
		LocalPubkeyHash:    "3722834BCB13F7308C28907B69A99DB462F39036",
	}
	conf.KeyStore = keyConf

	dgw := config.DgateWay{
		DBPath:        tmpDir,
		Count:         2,
		LocalID:       0,
		BchHeight:     100,
		EthHeight:     100,
		LocalP2PPort:  10000,
		LocalHTTPPort: 8080,
		Nodes: []config.Node{
			config.Node{
				Host:   "127.0.0.1:10000",
				Status: true,
				Pubkey: "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787",
			},
			config.Node{
				Host:   "127.0.0.1:10001",
				Status: true,
				Pubkey: "049FD6230E3BADBBC7BA190E10B2FC5C3D8EA9B758A43E98AB2C8F83C826AE7EABEA6D88880BC606FA595CD8DD17FC7784B3E55D8EE0705045119545A803215B80",
			},
		},
		EthConfirmCount: 6,
		EthClientURL:    "ws://47.98.185.203:8830",
		StartMode:       4,
	}
	conf.DgateWay = dgw

	conf.BCH = config.Server{
		RPCServer:               "47.97.167.221:8445",
		RPCUser:                 "tanshaohua",
		RPCPassword:             "hahaha",
		ConfirmBlockNum:         1,
		CoinbaseConfirmBlockNum: 100,
	}

	config.SetConf(conf)
	cluster.Init()
}

var p2pDB *p2pdb
var checkNode *node.BraftNode

func TestMain(m *testing.M) {
	tmpDir, err := ioutil.TempDir("", "p2p")
	if err != nil {
		panic("create tempdir failed")
	}
	//create p2pdb
	db, _ := dgwdb.NewLDBDatabase(tmpDir, 1, 1)
	p2pDB = &p2pdb{
		db: db,
	}
	nodeDir, _ := ioutil.TempDir("", "braft")
	initCuster(nodeDir)
	_, checkNode = node.RunNew(0, nil)
	defer checkNode.Stop()
	defer os.RemoveAll(nodeDir)
	defer os.RemoveAll(tmpDir)
	code := m.Run()
	defer os.Exit(code)
}
func TestProcessEmpty(t *testing.T) {
	// _, noderun := node.RunNew(0, nil)
	// defer noderun.Stop()
	tmpDir, _ := ioutil.TempDir("", "p2p")
	p2p := NewP2P(checkNode, tmpDir)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		p2p.ch <- &node.WatchedEvent{}
		p2p.ch <- &node.SignedEvent{}
		p2p.ch <- &node.ConfirmEvent{}
		p2p.ch <- &node.CommitedEvent{}
		close(p2p.ch)
		defer wg.Done()
	}()
	wg.Wait()
	p2p.processEvent()
}

func TestProcessMatch(t *testing.T) {
	// _, noderun := node.RunNew(0, nil)
	// defer noderun.Stop()
	tmpDir, _ := ioutil.TempDir("", "p2p")
	p2p := NewP2P(checkNode, tmpDir)

	initalEvent := &node.WatchedEvent{}
	requireAddr := getBytes(20)
	sendAddr := getBytes(20)
	sendAddr[0] = byte(255)
	requireAddr[0] = byte(254)

	initialData := &p2pMsg{
		SendAddr:    sendAddr,
		ReceiveAddr: getBytes(20),
		Chain:       1,
		TokenID:     1,
		Amount:      64,
		Fee:         1,
		ExpiredTime: uint32(time.Now().Unix()),
		RequireAddr: requireAddr,
	}
	initalEvent.Business = "p2p"
	txidInit := hex.EncodeToString(getBytes(32))
	initalEvent.Data = &pb.WatchedEvent{
		Business: "p2p",
		TxID:     txidInit,
		From:     2,
		Amount:   46,
		Data:     initialData.Encode(),
	}

	//matchEvent
	matchEvent := &node.WatchedEvent{}
	matchEvent.Business = "p2p"
	matchData := &p2pMsg{
		SendAddr:    requireAddr,
		ReceiveAddr: getBytes(20),
		Chain:       2,
		TokenID:     2,
		Amount:      46,
		Fee:         1,
		ExpiredTime: uint32(time.Now().Unix()),
		RequireAddr: sendAddr,
	}
	temp := getBytes(32)
	temp[0] = byte(2)
	txidMatch := hex.EncodeToString(temp)
	matchEvent.Data = &pb.WatchedEvent{
		Business: "p2p",
		TxID:     txidMatch,
		From:     1,
		Amount:   64,
		Data:     matchData.Encode(),
	}

	//do confirm
	initalEventConfirm := &node.ConfirmEvent{}
	id1, _ := hex.DecodeString(txidInit)
	initialDataConfirm := &p2pMsgConfirmed{
		ID:        id1,
		Chain:     1,
		Confirms:  7,
		Height:    10,
		BlockHash: getBytes(32),
		Amount:    1024,
		Fee:       1,
	}
	initalEventConfirm.Business = "p2p"
	initalEventConfirm.Data = &pb.WatchedEvent{
		Business: "p2p",
		TxID:     "initialNew",
		Data:     initialDataConfirm.Encode(),
		Amount:   46,
	}

	matchEventConfirm := &node.ConfirmEvent{}
	matchEventConfirm.Business = "p2p"
	id2, _ := hex.DecodeString(txidMatch)
	matchDataConfirm := &p2pMsgConfirmed{
		ID:        id2,
		Chain:     2,
		Confirms:  7,
		Height:    10,
		BlockHash: getBytes(32),
		Amount:    1024,
		Fee:       1,
	}
	matchEventConfirm.Data = &pb.WatchedEvent{
		TxID:     "matchNew",
		Business: "p2p",
		Data:     matchDataConfirm.Encode(),
		Amount:   64,
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		p2p.ch <- initalEvent
		// time.Sleep(2 * time.Second)
		p2p.ch <- matchEvent
		// p2p.ch <- matchEvent
		p2p.ch <- initalEventConfirm
		p2p.ch <- matchEventConfirm
		close(p2p.ch)
		defer wg.Done()
	}()

	p2p.processEvent()
	// ioutil.ReadAll(os.Stdout)
	wg.Wait()
}

func TestCreateDgw(t *testing.T) {
	info := &P2PInfo{}
	infos := []*P2PInfo{
		info,
	}
	confirmInfo := &P2PConfirmInfo{}
	confirmInfos := []*P2PConfirmInfo{
		confirmInfo,
	}
	innerTx := createDGWTx("test", infos, confirmInfos)
	fmt.Printf("preInit:%s,preMatch:%s", innerTx.Vin[0].TxID, innerTx.Vin[1].TxID)
	fmt.Printf("nowInit:%s,nowMatch:%s", innerTx.Vout[0].TxID, innerTx.Vout[1].TxID)
}

func TestIndex(t *testing.T) {
	root := &TxNode{
		Value:  "root",
		Childs: make(map[string]*TxNode),
	}
	index := &txIndex{
		root,
	}
	requireAddr := getBytes(20)
	p2pMsg := &p2pMsg{
		SendAddr:    getBytes(20),
		ReceiveAddr: getBytes(20),
		Chain:       1,
		TokenID:     1,
		Amount:      64,
		Fee:         1,
		ExpiredTime: uint32(time.Now().Unix()),
		RequireAddr: requireAddr,
	}
	msgUse := p2pMsg.toPBMsg()
	msgUse.SendAddr = []byte("sendAddr")
	event := &pb.WatchedEvent{
		TxID:   "testTxID",
		Amount: 1,
		From:   1,
		To:     2,
		Data:   p2pMsg.Encode(),
	}
	p2pInfo := &P2PInfo{
		Event: event,
		Msg:   msgUse,
		Index: 1,
	}
	index.Add(p2pInfo)
	p2pInfo.Event.TxID = "testTxID2"
	p2pInfo.Index = 0
	index.Add(p2pInfo)
	txIDs := index.GetTxID(1, string(msgUse.SendAddr), uint64(event.Amount))
	t.Logf("get txIDs:%s", txIDs)
	index.Del(p2pInfo)
	t.Logf("del res:%v", index.root.Childs)
	p2pInfo.Event.TxID = "testTxID"
	p2pInfo.Index = 1
	index.Del(p2pInfo)
	t.Logf("del res:%v", index.root.Childs)
}

func TestSignTimeout(t *testing.T) {
	service := newService(checkNode)
	sh := newSignedHandler(p2pDB, service, 1, 1)

	p2pDB.setWaitConfirm("init", &WaitConfirmMsg{
		ScTxId:   "1",
		Opration: confirmed,
		Chain:    10,
		Time:     time.Now().Unix(),
	})
	// sh.checkSignTimeout()
	// sh.runCheck()
	time.Sleep(2 * time.Second)
	go func() {
		for {
			se := &node.SignedEvent{}
			se.Data = &node.SignedData{
				Chain:          10,
				ID:             "1",
				TxID:           "1",
				SignBeforeTxID: "2",
			}
			sh.HandleEvent(se)
			time.Sleep(time.Second)
		}
	}()

	time.Sleep(5 * time.Second)
}

func TestConfirmTimeout(t *testing.T) {
	// service := newService(checkNode)
	// ch := newConfirmHandler(p2pDB, 1, service, 1)

	temp := getBytes(32)
	temp[0] = byte(2)
	txidMatch := hex.EncodeToString(temp)
	p2pDB.setSendedInfo(&SendedInfo{
		TxId:           txidMatch,
		SignTerm:       1,
		Chain:          10,
		SignBeforeTxId: "2",
		Time:           time.Now().Unix(),
	})

	// ch.runCheck()
	time.Sleep(10 * time.Second)
	// matchEvent := &node.ConfirmEvent{}
	// matchEvent.Business = "p2p"
	// matchData := &p2pMsg{
	// 	SendAddr:    getBytes(20),
	// 	ReceiveAddr: getBytes(20),
	// 	Chain:       2,
	// 	TokenID:     2,
	// 	Amount:      46,
	// 	Fee:         1,
	// 	ExpiredTime: uint32(time.Now().Unix()),
	// 	RequireAddr: getBytes(20),
	// }

	// matchEvent.Data = &pb.WatchedEvent{
	// 	Business: "p2p",
	// 	TxID:     txidMatch,
	// 	From:     1,
	// 	Amount:   64,
	// 	Data:     matchData.Encode(),
	// }

	// ch.HandleEvent(matchEvent)
	ids := []string{"1", "2"}
	p2pLogger.Debug("test", "ids", ids)
}
