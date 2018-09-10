package config_test

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/config"

	"github.com/spf13/viper"
)

func TestConfig(t *testing.T) {
	v := viper.New()
	configFile := "config.toml"
	v.SetConfigFile(configFile)
	err := v.ReadInConfig()
	if err != nil {
		t.Error(err)
	}
	config := &config.Config{}
	err = v.Unmarshal(config)
	if err != nil {
		t.Error(err)
	}
	v.WatchConfig()
	for _, node := range config.DgateWay.Nodes {
		fmt.Printf("testNode:%s,status:%t\n", node.Host, node.Status)
	}
	fmt.Printf("get:%s\n", config.DgateWay.NewNodeHost)
	fmt.Println("---------!")
	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Printf("name:%s\n", e.Name)
		v.Unmarshal(config)
	})
	go func() {
		ticker := time.NewTicker(time.Second).C
		for {
			<-ticker
			fmt.Println(config.DgateWay.NewNodeHost)
		}
	}()
	fmt.Println("---------")
	time.Sleep(120 * time.Second)
}

func TestInit(t *testing.T) {
	configFile := "config.toml"
	config.InitConf(configFile)
	go func() {
		ticker := time.NewTicker(time.Second).C
		for {
			<-ticker
			dgwConf := config.GetDGWConf()
			i := len(dgwConf.Nodes) - 1
			fmt.Printf("host:%s,cnt:%d\n", config.GetDGWConf().Nodes[i].Host, len(dgwConf.Nodes))
			metric := config.GetConf().Metrics
			fmt.Printf("metrics.interval:%d\n", metric.Interval)
		}
	}()
	time.AfterFunc(2*time.Second, func() {
		fmt.Println("set viper key")

		// Set("dgw.nodes", []map[string]interface{}{
		// 	{"host": "123", "status": true, "pubkey": "test"},
		// })

		conf := config.GetDGWConf()
		i := len(conf.Nodes) - 1
		fmt.Printf("after set viper host:%s,cnt:%d\n", config.GetDGWConf().Nodes[i].Host, len(conf.Nodes))
		// fmt.Println(v.Get("DGW.nodes"))
	})

	time.Sleep(120 * time.Second)
}

func saveNewConfig(localId int32) {
	confs := make(map[string]interface{})
	confs["KEYSTORE.count"] = cluster.TotalNodeCount
	confs["DGW.count"] = cluster.TotalNodeCount
	confs["DGW.local_id"] = localId
	if 2 == cluster.ModeJoin {
		confs["DGW.start_mode"] = 1
	}
	nodeConfs := make([]map[string]interface{}, 0)
	for _, nodeInfo := range cluster.NodeList {
		nodeConf := map[string]interface{}{
			"host":   nodeInfo.Name,
			"status": nodeInfo.IsNormal,
			"pubkey": hex.EncodeToString(nodeInfo.PublicKey),
		}
		nodeConfs = append(nodeConfs, nodeConf)
	}
	confs["dgw.nodes"] = nodeConfs
	confs["DGW.new_node_host"] = ""
	confs["DGW.new_node_pubkey"] = ""
	config.Set(confs)
}

func TestSaveConf(t *testing.T) {
	t.Log("---------------")
	cluster.TotalNodeCount = 5
	cluster.NodeList = []cluster.NodeInfo{
		cluster.NodeInfo{
			Id:        0,
			Name:      "server0",
			Url:       "127.0.0.1:10000",
			PublicKey: []byte("server0key"),
			IsNormal:  true,
		},
		cluster.NodeInfo{
			Id:        1,
			Name:      "server1",
			Url:       "127.0.0.1:10001",
			PublicKey: []byte("server1key"),
			IsNormal:  true,
		},
	}
	configFile := "config.toml"
	t.Log("---------------")
	config.InitConf(configFile)
	time.AfterFunc(2*time.Second, func() {
		saveNewConfig(0)
		newConf := config.GetConf()
		for _, node := range newConf.DgateWay.Nodes {
			fmt.Printf("host:%s,key:%s,status:%t\n", node.Host, node.Pubkey, node.Status)
		}
		fmt.Printf("newHost:%s,newKey:%s", newConf.DgateWay.NewNodeHost, newConf.DgateWay.NewNodePubkey)
	})
	time.Sleep(2 * time.Minute)
}
