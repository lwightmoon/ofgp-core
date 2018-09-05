package config

import (
	"fmt"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	NetParam   string     `mapstructure:"net_param"`
	LogLevel   string     `mapstructure:"loglevel"`
	BCH        Server     `mapstructure:"BCH"`
	BTC        Server     `mapstructure:"BTC"`
	DB         DB         `mapstructure:"LEVELDB"`
	KeyStore   KeyStore   `mapstructure:"KEYSTORE"`
	DgateWay   DgateWay   `mapstructure:"DGW"`
	Metrics    Metrics    `mapstructure:"METRICS"`
	EthWatcher EthWatcher `mapstructure:"ETHWATCHER"`
}

type Server struct {
	RPCServer               string `mapstructure:"rpc_server"`
	RPCUser                 string `mapstructure:"rpc_user"`
	RPCPassword             string `mapstructure:"rpc_password"`
	ConfirmBlockNum         int    `mapstructure:"confirm_block_num"`
	CoinbaseConfirmBlockNum int    `mapstructure:"coinbase_confirm_block_num"`
}

func checkServer(server *Server) {
	if server.RPCServer == "" {
		panic("rpc server host not set")
	}
}

type DB struct {
	BtcDBPath     string `mapstructure:"btc_db_path"`
	BchDBPath     string `mapstructure:"bch_db_path"`
	EwNonceDBPath string `mapstructure:"ew_nonce_db_path"`
}

func checkDB(db *DB) {
	if db.BchDBPath == "" {
		panic("bch db path not set")
	}
	if db.BtcDBPath == "" {
		panic("btc db path not set")
	}
	if db.EwNonceDBPath == "" {
		panic("ew nonce path not set")
	}
}

type KeyStore struct {
	URL                string `mapstructure:"url"`
	LocalPubkeyHash    string `mapstructure:"local_pubkey_hash"`
	Count              int    `mapstructure:"count"`
	ServiceID          string `mapstructure:"service_id"`
	KeyStorePrivateKey string `mapstructure:"keystore_private_key"`
}

func checkKeyStore(conf *KeyStore) {
	if conf.URL == "" {
		panic("key store url not set")
	}
	if conf.Count == 0 {
		panic("key store count is zero")
	}
	if conf.ServiceID == "" {
		panic("service id not set")
	}
	if conf.KeyStorePrivateKey == "" {
		panic("keystore private key not set")
	}
}

type DgateWay struct {
	Count             int           `mapstructure:"count"`
	LocalID           int32         `mapstructure:"local_id"`
	LocalP2PPort      int           `mapstructure:"local_p2p_port"`
	LocalHTTPPort     int           `mapstructure:"local_http_port"`
	LocalHTTPUser     string        `mapstructure:"local_http_user"`
	LocalHTTPPwd      string        `mapstructure:"local_http_pwd"`
	Nodes             []Node        `mapstructure:"nodes"`
	PProfHost         string        `mapstructure:"pprof_host"`
	NewNodeHost       string        `mapstructure:"new_node_host"`
	NewNodePubkey     string        `mapstructure:"new_node_pubkey"`
	BchHeight         int64         `mapstructure:"bch_height"`
	BtcHeight         int64         `mapstructure:"btc_height"`
	EthHeight         int64         `mapstructure:"eth_height"`
	DBPath            string        `mapstructure:"dbpath"`
	EthClientURL      string        `mapstructure:"eth_client_url"`
	EthConfirmCount   int64         `mapstructure:"eth_confirm_count"`
	EthTranIdx        int           `mapstructure:"eth_tran_idx"`
	StartMode         int32         `mapstructure:"start_mode"`
	InitNodeHost      string        `mapstructure:"init_node_host"` //节点join引导节点
	LocalHost         string        `mapstructure:"local_host"`
	LocalPubkey       string        `mapstructure:"local_pubkey"`
	BtcConfirms       int           `mapstructure:"btc_confirms"`
	BchConfirms       int           `mapstructure:"bch_confirms"`
	EthConfirms       int           `mapstructure:"eth_confirms"`
	ConfirmTolerance  time.Duration `mapstructure:"confirm_tolerance"`
	AccuseInterval    int64         `mapstructure:"accuse_interval"`
	UtxoLockTime      int           `mapstructure:"utxo_lock_time"`
	TxConnPoolSize    int           `mapstructure:"tx_conn_pool_size"`
	BlockConnPoolSize int           `mapstructure:"block_coon_pool_size"`
}

func checkDGWConf(conf *DgateWay) {
	if conf.Count == 0 {
		panic("count is zero")
	}
	if conf.LocalHTTPPort == 0 {
		panic("local http port not set")
	}
	if len(conf.Nodes) == 0 {
		panic("node confs is empty")
	}
	for _, node := range conf.Nodes {
		checkNode(&node)
	}
	if conf.BchHeight == 0 {
		panic("bch heigh not set")
	}
	if conf.BtcHeight == 0 {
		panic("bch heigh not set")
	}
	if conf.EthHeight == 0 {
		panic("bch heigh not set")
	}
	if conf.DBPath == "" {
		panic("db path not set")
	}
	if conf.EthClientURL == "" {
		panic("eth_client_url not set")
	}
}

type Node struct {
	Host   string `mapstructure:"host"`
	Status bool   `mapstructure:"status"`
	Pubkey string `mapstructure:"pubkey"`
}

func checkNode(node *Node) {
	if node.Host == "" {
		panic("node host not set")
	}
	if node.Pubkey == "" {
		panic("node pubkey not set")
	}
}

type Metrics struct {
	NeedMetric  bool          `mapstructure:"need_metric"`
	Interval    time.Duration `mapstructure:"interval"`
	InfluxdbURI string        `mapstructure:"influxdb_uri"`
	DB          string        `mapstructure:"db"`
	User        string        `mapstructure:"user"`
	Password    string        `mapstructure:"password"`
}

func checkMetrics(metric *Metrics) {
	if metric.NeedMetric {
		if metric.InfluxdbURI == "" {
			panic("influxdb uri not set")
		}
		if metric.DB == "" {
			panic("influxdb db not set")
		}
	}
}

type EthWatcher struct {
	VoteContract string `mapstructure:"vote_contract"`
}

func checkEthWatcher(watcher *EthWatcher) {
	if watcher.VoteContract == "" {
		panic("vote contract not set")
	}
}

var (
	conf *Config
	v    *viper.Viper
	lock sync.Mutex
)

func init() {
	v = viper.New()
}

func check(conf *Config) {
	checkServer(&conf.BCH)
	checkServer(&conf.BTC)
	checkDB(&conf.DB)
	checkKeyStore(&conf.KeyStore)
	checkDGWConf(&conf.DgateWay)
	checkEthWatcher(&conf.EthWatcher)
	checkMetrics(&conf.Metrics)
}

func InitConf(path string) {
	v.SetConfigFile(path)
	err := v.ReadInConfig()
	if err != nil {
		panic("read conf err:" + err.Error())
	}
	conf = &Config{}
	err = v.Unmarshal(conf)
	check(conf)
	if err != nil {
		panic("marshal conf err:" + err.Error())
	}
	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		newConf := &Config{}
		lock.Lock()
		err := v.Unmarshal(newConf)
		if err != nil {
			fmt.Printf("conf unmarshal err:%v", err)
			return
		}
		conf = newConf
		lock.Unlock()
		fmt.Println("config change")
		check(conf)
	})
}

func GetConf() *Config {
	lock.Lock()
	defer lock.Unlock()
	return conf
}

func GetKeyStoreConf() KeyStore {
	lock.Lock()
	defer lock.Unlock()
	return conf.KeyStore
}

func GetDGWConf() DgateWay {
	lock.Lock()
	defer lock.Unlock()
	return conf.DgateWay
}

func Set(confs map[string]interface{}) {
	lock.Lock()
	defer lock.Unlock()
	for key, value := range confs {
		v.Set(key, value)
	}
	newConf := &Config{}
	err := v.Unmarshal(newConf)
	if err != nil {
		fmt.Printf("unmarshal log err:%v", err)
		return
	}
	conf = newConf
	newConfName := "new_" + v.ConfigFileUsed()
	err = v.WriteConfigAs(newConfName)
	if err != nil {
		fmt.Printf("write new conf file err:%v", err)
	}
}

// SetNotWrite 保存配置不写磁盘
func SetNotWrite(confs map[string]interface{}) error {
	lock.Lock()
	defer lock.Unlock()
	for key, value := range confs {
		v.Set(key, value)
	}
	newConf := &Config{}
	err := v.Unmarshal(newConf)
	if err != nil {
		fmt.Printf("unmarshal log err:%v", err)
		return err
	}
	conf = newConf
	return nil
}
