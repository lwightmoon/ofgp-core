package config

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
	Keys               []string `mapstructure:"keys"`
	ServiceID          string   `mapstructure:"service_id"`
	KeyStorePrivateKey string   `mapstructure:"keystore_private_key"`
}

type DgateWay struct {
	Count             int    `mapstructure:"count"`
	LocalID           int    `mapstructure:"local_id"`
	LocalP2PPort      int    `mapstructure:"local_p2p_port"`
	LocalHTTPPort     int    `mapstructure:"local_http_port"`
	LocalHTTPUser     string `mapstructure:"local_http_user"`
	LocalHTTPPwd      string `mapstructure:"local_http_pwd"`
	Nodes             []Node `mapstructure:"nodestest"`
	PProfHost         string `mapstructure:"pprof_host"`
	NewNodeHost       string `mapstructure:"new_node_host"`
	NewNodePubkey     string `mapstructure:"new_node_pubkey"`
	BchHeight         int    `mapstructure:"bch_height"`
	BtcHeight         int    `mapstructure:"btc_height"`
	EthHeight         int    `mapstructure:"eth_height"`
	DBPath            string `mapstructure:"dbpath"`
	EthClientURL      string `mapstructure:"eth_client_url"`
	StartMode         int    `mapstructure:"start_mode"`
	InitNodeHost      string `mapstructure:"init_node_host"`
	LocalHost         string `mapstructure:"local_host"`
	LocalPubkey       string `mapstructure:"local_pubkey"`
	BtcConfirms       int    `mapstructure:"btc_confirms"`
	BchConfirms       int    `mapstructure:"bch_confirms"`
	ConfirmTolerance  int    `mapstructure:"confirm_tolerance"`
	AccuseInterval    int    `mapstructure:"accuse_interval"`
	UtxoLockTime      int    `mapstructure:"utxo_lock_time"`
	TxConnPoolSize    int    `mapstructure:"tx_conn_pool_size"`
	BlockConnPoolSize int    `mapstructure:"block_coon_pool_size"`
}

type Node struct {
	Host   string `mapstructure:"host"`
	Status bool   `mapstructure:"status"`
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
