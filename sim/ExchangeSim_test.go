package sim

import (
	"testing"
	"time"

	goexmodel "github.com/nntaoli-project/goex/v2/model"
	backtestmodel "github.com/nntaoli-project/goex_backtest/model"
	"github.com/stretchr/testify/assert"
)

func TestExchangeSimCreateOrderRejectsInsufficientBalance(t *testing.T) {
	pair := goexmodel.CurrencyPair{Symbol: "BTCUSDT", BaseSymbol: "BTC", QuoteSymbol: "USDT"}
	ex := NewExchangeSim(backtestmodel.ExchangeSimConfig{ExName: "test", QuoteCurrency: "USDT", SupportCurrencyPairs: []goexmodel.CurrencyPair{pair}, Accounts: map[string]goexmodel.Account{"USDT": {Coin: "USDT", Balance: 1, AvailableBalance: 1}}, BackTestStartTime: time.Now(), BackTestEndTime: time.Now(), DepthSize: 1})
	_, _, err := ex.CreateOrder(pair, 1, 2, goexmodel.Spot_Buy, goexmodel.OrderType_Limit)
	assert.ErrorIs(t, err, InsufficientError)
}
