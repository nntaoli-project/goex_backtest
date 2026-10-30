package loader

import (
	goexmodel "github.com/nntaoli-project/goex/v2/model"
	"github.com/nntaoli-project/goex_backtest/model"
	"testing"
)

func TestKLineDataLoaderNextWithoutData(t *testing.T) {
	loader := NewKLineDataLoader(model.DataConfig{})
	_, err := loader.Next(goexmodel.CurrencyPair{Symbol: "BTCUSDT"}, goexmodel.Kline_1min, 1)
	if err == nil {
		t.Fatal("expected missing data error")
	}
}
