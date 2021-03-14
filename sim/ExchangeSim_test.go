package sim

import (
	"fmt"
	"github.com/nntaoli-project/goex"
	"github.com/nntaoli-project/goex_backtest/model"
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
	"time"
)

var sim = NewExchangeSim(model.ExchangeSimConfig{
	ExName:               goex.BINANCE,
	TakerFee:             0.0002,
	MakerFee:             -0.0001,
	QuoteCurrency:        goex.USDT,
	SupportCurrencyPairs: []goex.CurrencyPair{goex.BTC_USDT},
	Account: goex.Account{
		SubAccounts: map[goex.Currency]goex.SubAccount{
			goex.BTC: goex.SubAccount{
				Currency: goex.BTC,
				Amount:   1,
			},
			goex.USDT: goex.SubAccount{
				Currency: goex.USDT,
				Amount:   100000,
			},
		},
	},
	BackTestStartTime: time.Date(2020, 03, 12, 0, 0, 0, 0, time.Local),
	BackTestEndTime:   time.Date(2020, 03, 12, 0, 0, 0, 0, time.Local),
	UnGzip:            false,
})

func TestExchangeSim_GetAccount(t *testing.T) {
	acc, _ := sim.GetAccount()
	assert.Equal(t, 1.0, acc.SubAccounts[goex.BTC].Amount)
	assert.Equal(t, 100000.0, acc.SubAccounts[goex.USDT].Amount)
}

func TestExchangeSim_GetDepth(t *testing.T) {
	t.Log(sim.GetDepth(5, goex.BTC_USDT)) //回测size参数不起作用的
}

func TestExchangeSim_LimitBuy(t *testing.T) {
	ord, _ := sim.LimitBuy("0.1", "7000", goex.BTC_USDT)
	t.Log(ord)
	acc, _ := sim.GetAccount()
	t.Log(acc)
	assert.Equal(t, 700.0, acc.SubAccounts[goex.USDT].ForzenAmount)
}

func TestExchangeSim_LimitSell(t *testing.T) {
	ord, _ := sim.LimitSell("0.1", "8000", goex.BTC_USDT)
	t.Log(ord)
	acc, _ := sim.GetAccount()
	t.Log(acc)
	assert.Equal(t, 0.1, acc.SubAccounts[goex.BTC].ForzenAmount)
}

func TestExchangeSim_LimitSell2(t *testing.T) {
	dep, _ := sim.GetDepth(5, goex.BTC_USDT)
	bidPrice := dep.BidList[0].Price
	ord, _ := sim.LimitSell("0.01", fmt.Sprint(bidPrice), goex.BTC_USDT)
	t.Log(ord)
	acc, _ := sim.GetAccount()
	expectedUsdt := 100000 + bidPrice*0.01 - sim.takerFee*bidPrice*0.01
	expectedBtc := 0.99
	assert.Equal(t, expectedBtc, acc.SubAccounts[goex.BTC].Amount)
	assert.Equal(t, expectedUsdt, acc.SubAccounts[goex.USDT].Amount)
}

func TestExchangeSim_CancelOrder(t *testing.T) {
	dep, _ := sim.GetDepth(5, goex.BTC_USDT)
	bid := dep.BidList[0]
	t.Log(bid)

	ord, _ := sim.LimitSell(fmt.Sprint(bid.Amount+0.01), fmt.Sprint(bid.Price), goex.BTC_USDT)
	t.Log(ord)
	assert.Equal(t, goex.ORDER_PART_FINISH, ord.Status)

	acc, _ := sim.GetAccount()
	expectedFrozenAmount := 0.01
	actualFrozenAmount := math.Round(acc.SubAccounts[goex.BTC].ForzenAmount*10000) / 10000
	assert.Equal(t, expectedFrozenAmount, actualFrozenAmount)

	expectedUsdt := math.Round((100000+bid.Price*bid.Amount-bid.Price*bid.Amount*sim.takerFee)*10000) / 10000
	actualUsdt := math.Round(acc.SubAccounts[goex.USDT].Amount*10000) / 10000
	assert.Equal(t, expectedUsdt, actualUsdt)

	t.Log(acc)

	_, err := sim.CancelOrder(ord.OrderID2, goex.BTC_USDT)
	assert.Equal(t, nil, err)

	ord, _ = sim.GetOneOrder(ord.OrderID2, goex.BTC_USDT)
	assert.Equal(t, goex.ORDER_CANCEL, ord.Status)

	acc, _ = sim.GetAccount()
	t.Log(acc)
	assert.Equal(t, 0.0, acc.SubAccounts[goex.BTC].ForzenAmount)

	expectedBtc := 1.0 - bid.Amount
	assert.Equal(t, expectedBtc, acc.SubAccounts[goex.BTC].Amount)
}
