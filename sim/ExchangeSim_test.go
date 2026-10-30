package sim

import (
	"errors"
	"sync"
	"testing"
	"time"

	goexmodel "github.com/nntaoli-project/goex/v2/model"
	backtestmodel "github.com/nntaoli-project/goex_backtest/model"
	"github.com/nntaoli-project/goex_backtest/util"
	"github.com/stretchr/testify/assert"
)

var testPair = goexmodel.CurrencyPair{Symbol: "BTCUSDT", BaseSymbol: "BTC", QuoteSymbol: "USDT"}

func newTestSim(dataType backtestmodel.BackTestDataType, makerFee, takerFee float64, accounts map[string]goexmodel.Account) *ExchangeSim {
	return &ExchangeSim{
		RWMutex:          new(sync.RWMutex),
		accounts:         accounts,
		name:             "test",
		makerFee:         makerFee,
		takerFee:         takerFee,
		quoteCurrency:    "USDT",
		pendingOrders:    make(map[string]*goexmodel.Order),
		finishedOrders:   make(map[string]*goexmodel.Order),
		idGen:            util.NewIdGen("test"),
		backTestDataType: dataType,
	}
}

func testAccounts(btc, usdt float64) map[string]goexmodel.Account {
	return map[string]goexmodel.Account{
		"BTC":  {Coin: "BTC", Balance: btc, AvailableBalance: btc},
		"USDT": {Coin: "USDT", Balance: usdt, AvailableBalance: usdt},
	}
}

func TestExchangeSimCreateOrderRejectsInsufficientBalance(t *testing.T) {
	ex := newTestSim(backtestmodel.BackTestDataType_KLine, 0.0001, 0.0003, testAccounts(0, 1))
	ex.currKline = goexmodel.Kline{Pair: testPair, Timestamp: 100, Close: 2}

	_, _, err := ex.CreateOrder(testPair, 1, 2, goexmodel.Spot_Buy, goexmodel.OrderType_Limit)

	assert.True(t, errors.Is(err, InsufficientError))
	assert.Empty(t, ex.pendingOrders)
	assert.Equal(t, 1.0, ex.accounts["USDT"].AvailableBalance)
}

func TestExchangeSimRejectsMarketOrder(t *testing.T) {
	ex := newTestSim(backtestmodel.BackTestDataType_KLine, 0.0001, 0.0003, testAccounts(0, 10000))
	ex.currKline = goexmodel.Kline{Pair: testPair, Timestamp: 100, Open: 100, High: 110, Low: 90, Close: 105, Vol: 10}

	_, _, err := ex.CreateOrder(testPair, 1, 0, goexmodel.Spot_Buy, goexmodel.OrderType_Market)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only limit orders")
	assert.Empty(t, ex.pendingOrders)
	assert.Equal(t, 10000.0, ex.accounts["USDT"].AvailableBalance)
	assert.Equal(t, 0.0, ex.accounts["USDT"].FrozenBalance)
}

func TestExchangeSimRejectsNonPositiveLimitPrice(t *testing.T) {
	ex := newTestSim(backtestmodel.BackTestDataType_KLine, 0.0001, 0.0003, testAccounts(0, 10000))
	ex.currKline = goexmodel.Kline{Pair: testPair, Timestamp: 100, Close: 100}

	_, _, err := ex.CreateOrder(testPair, 1, 0, goexmodel.Spot_Buy, goexmodel.OrderType_Limit)

	assert.Error(t, err)
	assert.Empty(t, ex.pendingOrders)
	assert.Equal(t, 10000.0, ex.accounts["USDT"].AvailableBalance)
}

func TestExchangeSimKLineLimitBuyMatchesNextCandle(t *testing.T) {
	ex := newTestSim(backtestmodel.BackTestDataType_KLine, 0.0005, 0.001, testAccounts(0, 10000))
	ex.currKline = goexmodel.Kline{Pair: testPair, Timestamp: 100, Open: 100, High: 105, Low: 99, Close: 100, Vol: 10}

	order, _, err := ex.CreateOrder(testPair, 2, 100, goexmodel.Spot_Buy, goexmodel.OrderType_Limit)

	assert.NoError(t, err)
	// 当根K线创建的订单不撮合，等待下一根K线
	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_Pending), order.Status)
	assert.InDelta(t, 200.0, ex.accounts["USDT"].FrozenBalance, 1e-12)

	ex.currKline = goexmodel.Kline{Pair: testPair, Timestamp: 200, Open: 99, High: 105, Low: 98, Close: 101, Vol: 10}
	ex.match()

	order, _, err = ex.GetOrderInfo(testPair, order.Id)
	assert.NoError(t, err)
	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_Finished), order.Status)
	assert.Equal(t, 2.0, order.ExecutedQty)
	// 下一根K线开盘价低于限价，按开盘价成交
	assert.Equal(t, 99.0, order.PriceAvg)
	// 挂单成交按 maker 费率，买入手续费扣基础币
	assert.InDelta(t, 2*0.0005, order.Fee, 1e-12)
	assert.Equal(t, "BTC", order.FeeCcy)
	assert.Empty(t, ex.pendingOrders)

	assert.InDelta(t, 2-0.001, ex.accounts["BTC"].Balance, 1e-12)
	assert.InDelta(t, 10000-198, ex.accounts["USDT"].Balance, 1e-12)
	assert.InDelta(t, 10000-198, ex.accounts["USDT"].AvailableBalance, 1e-12)
	assert.InDelta(t, 0.0, ex.accounts["USDT"].FrozenBalance, 1e-12)
}

func TestExchangeSimKLineLimitSellMatchesNextCandle(t *testing.T) {
	ex := newTestSim(backtestmodel.BackTestDataType_KLine, 0.0005, 0.001, testAccounts(2, 0))
	ex.currKline = goexmodel.Kline{Pair: testPair, Timestamp: 100, Open: 110, High: 112, Low: 108, Close: 110, Vol: 10}

	order, _, err := ex.CreateOrder(testPair, 1, 110, goexmodel.Spot_Sell, goexmodel.OrderType_Limit)

	assert.NoError(t, err)
	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_Pending), order.Status)
	assert.InDelta(t, 1.0, ex.accounts["BTC"].FrozenBalance, 1e-12)

	ex.currKline = goexmodel.Kline{Pair: testPair, Timestamp: 200, Open: 111, High: 112, Low: 109, Close: 110, Vol: 10}
	ex.match()

	order, _, err = ex.GetOrderInfo(testPair, order.Id)
	assert.NoError(t, err)
	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_Finished), order.Status)
	assert.Equal(t, 1.0, order.ExecutedQty)
	// 下一根K线开盘价高于限价，按开盘价成交
	assert.Equal(t, 111.0, order.PriceAvg)
	assert.InDelta(t, 111*0.0005, order.Fee, 1e-12)
	assert.Equal(t, "USDT", order.FeeCcy)

	assert.InDelta(t, 1.0, ex.accounts["BTC"].Balance, 1e-12)
	assert.InDelta(t, 0.0, ex.accounts["BTC"].FrozenBalance, 1e-12)
	assert.InDelta(t, 111-111*0.0005, ex.accounts["USDT"].Balance, 1e-12)
}

func TestExchangeSimKLineLimitOrderOnlyMatchesNextCandle(t *testing.T) {
	ex := newTestSim(backtestmodel.BackTestDataType_KLine, 0.0005, 0.001, testAccounts(0, 10000))
	ex.currKline = goexmodel.Kline{Pair: testPair, Timestamp: 100, Open: 100, High: 105, Low: 99, Close: 100, Vol: 10}

	order, _, err := ex.CreateOrder(testPair, 1, 100, goexmodel.Spot_Buy, goexmodel.OrderType_Limit)
	assert.NoError(t, err)

	// 同一根K线即使价格触及也不成交
	ex.match()
	order, _, _ = ex.GetOrderInfo(testPair, order.Id)
	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_Pending), order.Status)

	// 下一根K线价格未触及限价，继续挂单
	ex.currKline = goexmodel.Kline{Pair: testPair, Timestamp: 200, Open: 102, High: 106, Low: 101, Close: 105, Vol: 10}
	ex.match()
	order, _, _ = ex.GetOrderInfo(testPair, order.Id)
	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_Pending), order.Status)
	assert.InDelta(t, 100.0, ex.accounts["USDT"].FrozenBalance, 1e-12)

	// 再下一根K线价格触及，成交
	ex.currKline = goexmodel.Kline{Pair: testPair, Timestamp: 300, Open: 101, High: 104, Low: 99, Close: 102, Vol: 10}
	ex.match()
	order, _, _ = ex.GetOrderInfo(testPair, order.Id)
	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_Finished), order.Status)
	assert.Equal(t, 100.0, order.PriceAvg)
}

func TestExchangeSimKLineVolumeLimitsFill(t *testing.T) {
	ex := newTestSim(backtestmodel.BackTestDataType_KLine, 0.0005, 0.001, testAccounts(0, 10000))
	ex.currKline = goexmodel.Kline{Pair: testPair, Timestamp: 100, Open: 100, High: 105, Low: 99, Close: 100, Vol: 10}

	order, _, err := ex.CreateOrder(testPair, 2, 100, goexmodel.Spot_Buy, goexmodel.OrderType_Limit)
	assert.NoError(t, err)

	// 下一根K线成交量只有1，只能部分成交
	ex.currKline = goexmodel.Kline{Pair: testPair, Timestamp: 200, Open: 100, High: 105, Low: 99, Close: 101, Vol: 1}
	ex.match()
	order, _, _ = ex.GetOrderInfo(testPair, order.Id)
	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_PartFinished), order.Status)
	assert.Equal(t, 1.0, order.ExecutedQty)
	assert.InDelta(t, 100.0, ex.accounts["USDT"].FrozenBalance, 1e-12)

	// 下一根K线继续成交剩余部分
	ex.currKline = goexmodel.Kline{Pair: testPair, Timestamp: 300, Open: 100, High: 105, Low: 99, Close: 101, Vol: 1}
	ex.match()
	order, _, _ = ex.GetOrderInfo(testPair, order.Id)
	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_Finished), order.Status)
	assert.Equal(t, 2.0, order.ExecutedQty)
	assert.InDelta(t, 0.0, ex.accounts["USDT"].FrozenBalance, 1e-12)
	assert.InDelta(t, 2-0.001, ex.accounts["BTC"].Balance, 1e-12)
}

func TestExchangeSimDepthLimitBuyWalksBook(t *testing.T) {
	ex := newTestSim(backtestmodel.BackTestDataType_Depth, 0.0005, 0.001, testAccounts(0, 10000))
	ex.currDepth = goexmodel.Depth{
		Pair:  testPair,
		UTime: time.Unix(100, 0),
		// 卖盘降序排列，最优卖价在数组末尾
		Asks: []goexmodel.DepthItem{
			{Price: 102, Amount: 1},
			{Price: 101, Amount: 1},
			{Price: 100, Amount: 0.5},
		},
		Bids: []goexmodel.DepthItem{{Price: 99, Amount: 1}},
	}

	order, _, err := ex.CreateOrder(testPair, 2, 101.5, goexmodel.Spot_Buy, goexmodel.OrderType_Limit)

	assert.NoError(t, err)
	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_PartFinished), order.Status)
	assert.InDelta(t, 1.5, order.ExecutedQty, 1e-12)
	assert.InDelta(t, (0.5*100+1*101)/1.5, order.PriceAvg, 1e-12)
	assert.InDelta(t, 1.5*0.001, order.Fee, 1e-12)
	assert.Equal(t, "BTC", order.FeeCcy)
	// 剩余 0.5 仍按限价冻结
	assert.InDelta(t, 0.5*101.5, ex.accounts["USDT"].FrozenBalance, 1e-12)

	// 下一档行情继续以 maker 费率成交剩余部分
	ex.currDepth = goexmodel.Depth{
		Pair:  testPair,
		UTime: time.Unix(101, 0),
		Asks:  []goexmodel.DepthItem{{Price: 101, Amount: 1}},
		Bids:  []goexmodel.DepthItem{{Price: 99, Amount: 1}},
	}
	ex.match()
	order, _, err = ex.GetOrderInfo(testPair, order.Id)
	assert.NoError(t, err)

	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_Finished), order.Status)
	assert.Equal(t, 2.0, order.ExecutedQty)
	assert.InDelta(t, 1.5*0.001+0.5*0.0005, order.Fee, 1e-12)
	assert.InDelta(t, 0.0, ex.accounts["USDT"].FrozenBalance, 1e-12)
	assert.InDelta(t, 10000-50-101-50.5, ex.accounts["USDT"].Balance, 1e-12)
	assert.InDelta(t, 2-(1.5*0.001+0.5*0.0005), ex.accounts["BTC"].Balance, 1e-12)
}

func TestExchangeSimPartialFillCompletesWithoutFloatDrift(t *testing.T) {
	ex := newTestSim(backtestmodel.BackTestDataType_Depth, 0.0005, 0.001, testAccounts(0, 10000))
	ex.currDepth = goexmodel.Depth{
		Pair:  testPair,
		UTime: time.Unix(100, 0),
		Asks: []goexmodel.DepthItem{
			{Price: 101, Amount: 0.2},
			{Price: 100, Amount: 0.1},
		},
		Bids: []goexmodel.DepthItem{{Price: 99, Amount: 1}},
	}

	order, _, err := ex.CreateOrder(testPair, 0.3, 101, goexmodel.Spot_Buy, goexmodel.OrderType_Limit)

	assert.NoError(t, err)
	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_Finished), order.Status)
	assert.Equal(t, 0.3, order.ExecutedQty)
	assert.Empty(t, ex.pendingOrders)
	assert.InDelta(t, 0.0, ex.accounts["USDT"].FrozenBalance, 1e-12)
}

func TestExchangeSimCancelLimitOrderReleasesFrozen(t *testing.T) {
	ex := newTestSim(backtestmodel.BackTestDataType_Depth, 0.0005, 0.001, testAccounts(0, 10000))
	ex.currDepth = goexmodel.Depth{
		Pair:  testPair,
		UTime: time.Unix(100, 0),
		Asks:  []goexmodel.DepthItem{{Price: 100, Amount: 1}},
		Bids:  []goexmodel.DepthItem{{Price: 99, Amount: 1}},
	}

	order, _, err := ex.CreateOrder(testPair, 1, 90, goexmodel.Spot_Buy, goexmodel.OrderType_Limit)
	assert.NoError(t, err)
	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_Pending), order.Status)
	assert.InDelta(t, 90.0, ex.accounts["USDT"].FrozenBalance, 1e-12)

	_, err = ex.CancelOrder(testPair, order.Id)
	assert.NoError(t, err)
	order, _, err = ex.GetOrderInfo(testPair, order.Id)
	assert.NoError(t, err)

	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_Canceled), order.Status)
	assert.InDelta(t, 10000.0, ex.accounts["USDT"].AvailableBalance, 1e-12)
	assert.InDelta(t, 0.0, ex.accounts["USDT"].FrozenBalance, 1e-12)
	assert.Empty(t, ex.pendingOrders)
}

func TestExchangeSimDepthLimitSellWalksBook(t *testing.T) {
	ex := newTestSim(backtestmodel.BackTestDataType_Depth, 0.0005, 0.001, testAccounts(2, 0))
	ex.currDepth = goexmodel.Depth{
		Pair:  testPair,
		UTime: time.Unix(100, 0),
		Asks:  []goexmodel.DepthItem{{Price: 101, Amount: 1}},
		// 买盘降序排列，最优买价在数组开头
		Bids: []goexmodel.DepthItem{
			{Price: 100, Amount: 1},
			{Price: 99, Amount: 2},
		},
	}

	order, _, err := ex.CreateOrder(testPair, 2, 99.5, goexmodel.Spot_Sell, goexmodel.OrderType_Limit)

	assert.NoError(t, err)
	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_PartFinished), order.Status)
	assert.InDelta(t, 1.0, order.ExecutedQty, 1e-12)
	assert.InDelta(t, 100.0, order.PriceAvg, 1e-12)
	assert.InDelta(t, 100*0.001, order.Fee, 1e-12)
	assert.Equal(t, "USDT", order.FeeCcy)
	assert.InDelta(t, 1.0, ex.accounts["BTC"].FrozenBalance, 1e-12)

	ex.currDepth = goexmodel.Depth{
		Pair:  testPair,
		UTime: time.Unix(101, 0),
		Asks:  []goexmodel.DepthItem{{Price: 101, Amount: 1}},
		Bids:  []goexmodel.DepthItem{{Price: 100, Amount: 2}},
	}
	ex.match()
	order, _, err = ex.GetOrderInfo(testPair, order.Id)
	assert.NoError(t, err)

	assert.Equal(t, goexmodel.OrderStatus(goexmodel.OrderStatus_Finished), order.Status)
	assert.Equal(t, 2.0, order.ExecutedQty)
	assert.InDelta(t, 100*0.001+100*0.0005, order.Fee, 1e-12)
	assert.InDelta(t, 0.0, ex.accounts["BTC"].FrozenBalance, 1e-12)
	assert.InDelta(t, 0.0, ex.accounts["BTC"].Balance, 1e-12)
	assert.InDelta(t, (100-0.1)+(100-0.05), ex.accounts["USDT"].Balance, 1e-12)
}
