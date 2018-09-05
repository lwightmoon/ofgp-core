package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/ofgp/ofgp-core/config"
	sg "github.com/ofgp/ofgp-core/util/signal"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/httpsvr"
	"github.com/ofgp/ofgp-core/node"
	"github.com/ofgp/ofgp-core/util"

	"github.com/ofgp/ofgp-core/accuser"

	"github.com/rcrowley/go-metrics"
	"github.com/vrischmann/go-metrics-influxdb"
	"gopkg.in/urfave/cli.v1"
)

var (
	app       = util.NewApp()
	signalSet = sg.NewSignalSet()
)

func init() {
	app.Action = run
	app.HideVersion = true
	app.Copyright = "Copyright"
	app.Commands = []cli.Command{}

	//app.Flags = append(app.Flags, util.P2PPortFlag)
	//app.Flags = append(app.Flags, util.DBPathFlag)
	//app.Flags = append(app.Flags, util.HTTPPortFlag)
	app.Flags = append(app.Flags, util.ConfigFileFlag)
	app.Flags = append(app.Flags, util.CPUProfileFlag)
	app.Flags = append(app.Flags, util.MemProfileFlag)
	//app.Flags = append(app.Flags, util.BchHeightFlag)
	for _, flag := range util.Flags {
		app.Flags = append(app.Flags, flag)
	}
}

func baseMetrics(conf config.Metrics) {
	interval := conf.Interval
	r := metrics.NewRegistry()

	metrics.RegisterDebugGCStats(r)
	go metrics.CaptureDebugGCStats(r, interval)
	metrics.RegisterRuntimeMemStats(r)
	go metrics.CaptureRuntimeMemStats(r, interval)

	g := metrics.NewGauge()
	r.Register("numgoroutine", g)
	go func() {
		for {
			g.Update(int64(runtime.NumGoroutine()))
			time.Sleep(interval)
		}
	}()

	go influxdb.InfluxDB(r, 10e9, conf.InfluxDBURI,
		conf.DB, conf.User,conf.Password)
}

func run(ctx *cli.Context) {
	configFile := util.GetConfigFile(ctx)

	//初始化配置
	config.InitConf(configFile)
	conf := config.GetConf()

	//todo 处理read 命令行参数到viper
	util.ReadConfigToViper(ctx)

	// 如果需要做性能检测
	cpuProfile := util.GetCPUProfile(ctx)
	if len(cpuProfile) > 0 {
		f, err := os.Create(cpuProfile)
		if err != nil {
			panic("create cpu profile failed")
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	memProfile := util.GetMemProfile(ctx)
	if len(memProfile) > 0 {
		f, err := os.Create(memProfile)
		if err != nil {
			panic("create mem profile failed")
		}
		defer pprof.WriteHeapProfile(f)
	}
	dgwConf := conf.DgateWay
	go func() {

		log.Println(http.ListenAndServe(dgwConf.PProfHost, nil))
		// log.Println(http.ListenAndServe(":8060", nil))
	}()

	nodeId := dgwConf.LocalID
	startMode := dgwConf.StartMode
	//设置btc bch 确认块
	node.BtcConfirms = dgwConf.BtcConfirms
	node.BchConfirms = dgwConf.BchConfirms
	node.EthConfirms = dgwConf.EthConfirms
	//交易处理超时时间
	node.ConfirmTolerance = dgwConf.ConfirmTolerance
	//设置发起accuse 的间隔
	accuser.AccuseInterval = dgwConf.AccuseInterval

	var joinMsg *node.JoinMsg
	if startMode == cluster.ModeNormal {
		cluster.Init()
	} else {
		joinMsg = node.InitJoin()
		nodeId = joinMsg.LocalID
	}

	httpPort := dgwConf.LocalHTTPPort

	cros := []string{}
	if nodeId < 0 || int(nodeId) >= len(cluster.NodeList) {
		panic(fmt.Sprintf("Invalid nodeid %d cluster size %d", nodeId, len(cluster.NodeList)))
	}

	var multiSigs []cluster.MultiSigInfo
	if joinMsg != nil && len(joinMsg.MultiSigInfos) > 0 {
		multiSigs = joinMsg.MultiSigInfos
	}
	_, node := node.RunNew(nodeId, multiSigs)

	user := dgwConf.LocalHTTPUser
	pwd := dgwConf.LocalHTTPPwd
	httpsvr.StartHTTP(node, user, pwd, fmt.Sprintf(":%d", httpPort), cros)

	metricConf := conf.Metrics
	needMetrics := metricConf.NeedMetric
	if needMetrics {
		baseMetrics(metricConf)
	}

	// 添加需要捕获的信号
	if startMode != cluster.ModeWatch && startMode != cluster.ModeTest {
		signalSet.Register(syscall.SIGINT, node.LeaveCluster)
	} else {
		// 观察节点只用自己退出就可以了，不用发LeaveRequest
		signalSet.Register(syscall.SIGINT, node.Stop)
	}
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGINT)
	sig := <-sigChan
	fmt.Printf("receive signal %v\n", sig)
	signalSet.Handle(sig)
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
