package price_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/ofgp/ofgp-core/price"
)

func TestGetCurrPrice(t *testing.T) {
	tool := price.NewPriceTool("http://127.0.0.1:8000")
	res, err := tool.GetCurrPrice("BCH-USDT", true)
	if err != nil {
		t.Error("get curr price failed, err: ", err)
	}
	if len(res.Err) != 0 {
		t.Error("get curr price failed, err: ", res.Err)
	} else {
		t.Logf("get price: %v %v", res.Price, res.Timestamp)
	}
}

func TestGetPriceByTs(t *testing.T) {
	tool := price.NewPriceTool("http://127.0.0.1:8000")
	res, err := tool.GetPriceByTimestamp("BCH-USDT", 1539344897, true)
	if err != nil {
		t.Error("get curr price failed, err: ", err)
	}
	if len(res.Err) != 0 {
		t.Error("get curr price failed, err: ", res.Err)
	} else {
		t.Logf("get price: %v %v", res.Price, res.Timestamp)
	}
}

func TestSendConfirm(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("hello world!")
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	t.Log(server.URL)
	tool := price.NewPriceTool(server.URL)
	for i := 0; i < 2; i++ {
		istr := strconv.Itoa(i)
		tool.SendConfirm(&price.ConfirmInfo{
			Txid: "txid" + istr,
		})
	}
	time.Sleep(2 * time.Second)
}

type apiData struct {
	Code int         `json:"code"`
	Err  string      `json:"err"`
	Data interface{} `json:"data"`
}

// Price price
type Price struct {
	ID        int     `json:"id"`    //报价id
	Price     float64 `json:"price"` //报价
	PriceType string  `json:"type"`  //ask/bid
}

func TestGetPriceByHash(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		price := &Price{
			ID:        1,
			Price:     8848.0,
			PriceType: "bid",
		}
		data := &apiData{
			Code: 200,
			Err:  "",
			Data: price,
		}
		databytes, _ := json.Marshal(data)
		w.Write(databytes)
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	tool := price.NewPriceTool(server.URL)
	price, err := tool.GetPriceByTxid("12345")
	if err != nil {
		t.Errorf("get price err:%v", err)
		return
	}
	t.Logf("get price:%v", price)
}
