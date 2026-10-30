package strategies

import (
	"context"
	"github.com/markcheno/go-talib"
	goexmodel "github.com/nntaoli-project/goex/v2/model"
	"github.com/nntaoli-project/goex_backtest/sim"
	"log"
)

type DoubleMovingStrategy struct {
	api         *sim.ExchangeSim
	pair        goexmodel.CurrencyPair
	period      goexmodel.KlinePeriod
	long, short int
	holdOrder   *goexmodel.Order
	closePrices []float64
}

func NewDoubleMovingStrategy(api *sim.ExchangeSim, period goexmodel.KlinePeriod, long, short int, pair goexmodel.CurrencyPair) *DoubleMovingStrategy {
	return &DoubleMovingStrategy{api: api, pair: pair, period: period, long: long, short: short}
}
func (s *DoubleMovingStrategy) Main(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			data, _, err := s.api.GetKline(s.pair, s.period)
			if err != nil {
				return
			}
			for _, kline := range data {
				s.closePrices = append(s.closePrices, kline.Close)
			}
			if len(s.closePrices) < s.long {
				continue
			}
			long := talib.Ema(s.closePrices, s.long)
			short := talib.Ema(s.closePrices, s.short)
			if short[len(short)-1] > long[len(long)-1] {
				if s.holdOrder == nil {
					order, _, err := s.api.CreateOrder(s.pair, 0.4, data[0].Close, goexmodel.Spot_Buy, goexmodel.OrderType_Limit)
					if err == nil {
						s.holdOrder = order
						s.api.AssetSnapshot()
						log.Printf("[开仓] 价格:%f", order.Price)
					}
				}
			} else if s.holdOrder != nil {
				order, _, err := s.api.CreateOrder(s.pair, s.holdOrder.Qty, data[0].Close, goexmodel.Spot_Sell, goexmodel.OrderType_Limit)
				if err == nil {
					s.api.AssetSnapshot()
					log.Printf("[平仓] 价格:%f", order.Price)
					s.holdOrder = nil
				}
			}
		}
	}
}
