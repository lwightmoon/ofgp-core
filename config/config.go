package config

import (
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

type DB struct {
	BtcDBPath     string `mapstructure:"btc_db_path"`
	BchDBPath     string `mapstructure:"bch_db_path"`
	EwNonceDBPath string `mapstructure:"ew_nonce_db_path"`
}

type KeyStore struct {
	URL                string   `mapstructure:"url"`
	LocalPubkeyHash    string   `mapstructure:"local_pubkey_hash"`
	Count              int      `mapstructure:"count"`
	Keys               []string `mapstructure:"keys"`
	ServiceID          string   `mapstructure:"service_id"`
	KeyStorePrivateKey string   `mapstructure:"keystore_private_key"`
}

type DgateWay struct {
	Count             int           `mapstructure:"count"`
	LocalID           int32         `mapstructure:"local_id"`
	LocalP2PPort      int           `mapstructure:"local_p2p_port"`
	LocalHTTPPort     int           `mapstructure:"local_http_port"`
	LocalHTTPUser     string        `mapstructure:"local_http_user"`
	LocalHTTPPwd      string        `mapstructure:"local_http_pwd"`
	Nodes             []Node        `mapstructure:"nodestest"`
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

type Node struct {
	Host   string `mapstructure:"host"`
	Status bool   `mapstructure:"status"`
	Pubkey string `mapstructure:"pubkey"`
}

type Metrics struct {
	NeedMetric  bool   `mapstructure:"need_metric"`
	Interval    int64  `mapstructure:"interval"`
	InfluxdbURI string `mapstructure:"influxdb_uri"`
	DB          string `mapstructure:"db"`
	User        string `mapstructure:"user"`
	Password    string `mapstructure:"password"`
}

type EthWatcher struct {
	VoteContract string `mapstructure:"vote_contract"`
}

var (
	conf *Config
	v    *viper.Viper
)

func init() {
	v = viper.New()
}

func InitConf(path string) {
	v.SetConfigFile(path)
	err := v.ReadInConfig()
	if err != nil {
		panic("read conf err:" + err.Error())
	}
	conf = &Config{}
	err = v.Unmarshal(conf)
	if err != nil {
		panic("marshal conf err:" + err.Error())
	}
	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		v.Unmarshal(conf)
	})
}

func GetConf() *Config {
	return conf
}

func GetKeyStoreConf() KeyStore {
	return conf.KeyStore
}

func GetDGWConf() DgateWay {
	return conf.DgateWay
}

// GetLogLevel 获取log级别
func GetLogLevel() string {
	return conf.LogLevel
}
