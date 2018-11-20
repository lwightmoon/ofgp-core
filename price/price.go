package price

import (
	"bytes"
	"container/list"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	pb "github.com/ofgp/ofgp-core/proto"
)

// PriceInfo 币价信息
type PriceInfo struct {
	Price     float32 `json:"price"`
	Timestamp int64   `json:"timestamp"`
	Err       string  `json:"err"`
}

// ConfirmInfo tx完成通知
type ConfirmInfo struct {
	Txid string `json:"txhash"`
}

// Price price
type Price struct {
	ID        int     `json:"id"`    //报价id
	Price     float64 `json:"price"` //报价
	PriceType string  `json:"type"`  //ask/bid
}

// PriceTool 获取币价的工具
type PriceTool struct {
	endpoint string
	client   *http.Client
	listLock sync.RWMutex
	sendList *list.List
}

// NewPriceTool 返回一个PriceTool实例
func NewPriceTool(endpoint string) *PriceTool {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   800 * time.Millisecond,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
		Timeout: 1000 * time.Millisecond,
	}

	tool := &PriceTool{
		endpoint: endpoint,
		client:   client,
		sendList: list.New(),
	}
	tool.run()
	return tool
}

func (t *PriceTool) dial(url string) (*PriceInfo, error) {
	resp, err := t.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	res := new(PriceInfo)
	err = json.Unmarshal(body, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// GetCurrPrice 获取指定交易对的币价，ex: BCH-USDT
func (t *PriceTool) GetCurrPrice(symbol string, forth bool) (*PriceInfo, error) {
	tmp, err := url.Parse(strings.Join([]string{t.endpoint, "currprice", symbol}, "/"))
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if forth {
		params.Set("forth", "1")
	} else {
		params.Set("forth", "0")
	}
	tmp.RawQuery = params.Encode()
	return t.dial(tmp.String())
}

// GetPriceByTimestamp 获取指定交易对某个时间戳的币价
func (t *PriceTool) GetPriceByTimestamp(symbol string, ts int64, forth bool) (*PriceInfo, error) {
	tmp, err := url.Parse(strings.Join([]string{t.endpoint, "pricebyts", symbol, strconv.FormatInt(ts, 10)}, "/"))
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if forth {
		params.Set("forth", "1")
	} else {
		params.Set("forth", "0")
	}
	tmp.RawQuery = params.Encode()
	return t.dial(tmp.String())
}

// GetPriceByTxid 根据txid获取币价
func (t *PriceTool) GetPriceByTxid(txid string) (*Price, error) {
	urlStr := strings.Join([]string{t.endpoint, "pricebyts"}, "/")
	tmp, err := url.Parse(urlStr)
	if err != nil {
		log.Printf("parse url err:%v", err)
		return nil, err
	}
	params := url.Values{}
	params.Set("txhash", txid)
	tmp.RawQuery = params.Encode()
	res, err := t.client.Get(tmp.String())
	if err != nil {
		log.Printf("get url:%s err:%v", tmp.String(), err)
		return nil, err
	}
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	price := &Price{}
	err = json.Unmarshal(body, price)
	if err != nil {
		log.Printf("unmarshal price info err:%v", err)
		return nil, err
	}
	return price, err
}

// SendConfirm 发送交易confirm消息
func (t *PriceTool) SendConfirm(confirmInfo *ConfirmInfo) {
	t.listLock.Lock()
	defer t.listLock.Unlock()
	t.sendList.PushBack(confirmInfo)
}

// OnNewBlockCommitted 区块提交通知交易处理结果
func (t *PriceTool) OnNewBlockCommitted(pack *pb.BlockPack) {
	block := pack.Block()
	if block != nil && len(block.Txs) > 0 {
		for _, tx := range block.Txs {
			confirmInfo := &ConfirmInfo{
				Txid: tx.WatchedTx.Txid,
			}
			t.SendConfirm(confirmInfo)
		}
	}
}

//NoticeConfirm 通知交易完成
func (t *PriceTool) noticeConfirm() {
	t.listLock.Lock()
	defer t.listLock.Unlock()
	element := t.sendList.Front()
	if element == nil {
		//list is empty sleep for a while
		time.Sleep(100 * time.Millisecond)
		return
	}
	confirmInfo := element.Value.(*ConfirmInfo)
	data, _ := json.Marshal(confirmInfo)
	body := bytes.NewBuffer(data)
	confirmURL := strings.Join([]string{t.endpoint, "confirm"}, "/")
	req, err := http.NewRequest("POST", confirmURL, body)
	if err != nil {
		log.Printf("create confirm http req err:%v", err)
		return
	}
	res, err := t.client.Do(req)
	if err != nil {
		t.sendList.PushBack(confirmInfo)
		//发送失败短暂暂停
		time.Sleep(100 * time.Millisecond)
		return
	}
	defer res.Body.Close()
}

func (t *PriceTool) run() {
	go func() {
		for {
			t.noticeConfirm()
		}
	}()
}
