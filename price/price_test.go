package price_test

import (
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
