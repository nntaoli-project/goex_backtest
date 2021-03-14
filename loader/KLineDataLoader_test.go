package loader

import (
	"github.com/nntaoli-project/goex"
	"github.com/nntaoli-project/goex_backtest/model"
	"testing"
	"time"
)

func TestKLineDataLoader(t *testing.T) {
	loader := NewKLineDataLoader(model.DataConfig{
		Ex:       "huobi.pro",
		StarTime: time.Date(2020, 3, 1, 0, 0, 0, 0, time.Local),
		EndTime:  time.Date(2020, 3, 3, 0, 0, 0, 0, time.Local),
	})

	for {
		klineData, err := loader.Next(goex.BTC_USDT, goex.KLINE_PERIOD_1MIN, 10)
		if err != nil {
			t.Log(err)
			return
		}
		t.Log(klineData)
	}
}
