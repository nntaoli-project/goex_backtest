package strategies

import (
	"context"
	"fmt"
	"github.com/markcheno/go-talib"
	"github.com/nntaoli-project/goex"
	"github.com/nntaoli-project/goex_backtest/sim"
	"github.com/nntaoli-project/goex_talib"
	"log"
)

type DoubleMovingStrategy struct {
	api    goex.API
	pair   goex.CurrencyPair
	period goex.KlinePeriod
	long   int
	short  int

	holdOder *goex.Order
}

func NewDoubleMovingStrategy(api goex.API,
	period goex.KlinePeriod,
	long, short int,
	pair goex.CurrencyPair) *DoubleMovingStrategy {
	return &DoubleMovingStrategy{
		api:    api,
		pair:   pair,
		period: period,
		long:   long,
		short:  short,
	}
}

func (s *DoubleMovingStrategy) Main(ctx context.Context) {
	api := s.api.(*sim.ExchangeSim)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			klineData, err := s.api.GetKlineRecords(s.pair, s.period, s.long)
			if err != nil {
				return
			}
			longValue := goex_talib.Ma(klineData, s.long, talib.EMA, goex_talib.InClose)
			shortValue := goex_talib.Ma(klineData, s.short, talib.EMA, goex_talib.InClose)
			if shortValue[len(shortValue)-1] > longValue[len(longValue)-1] {
				if s.holdOder == nil {
					s.holdOder, _ = api.LimitBuy("0.4", fmt.Sprint(klineData[0].Close), s.pair)
					api.AssetSnapshot()
					log.Printf("[开仓] 短期均线上穿长期均线,短期%d均线值:%f,长期%d均线值:%f,开仓价:%f", s.short, shortValue[len(shortValue)-1], s.long, longValue[len(longValue)-1], s.holdOder.Price)
				}
			} else {
				if s.holdOder != nil {
					ord, _ := api.LimitSell(fmt.Sprint(s.holdOder.Amount), fmt.Sprint(klineData[0].Close), s.pair)
					api.AssetSnapshot()
					log.Printf("[平仓] 短期均线下穿长期均线,短期%d均线值:%f,长期%d均线值:%f,平仓价:%f", s.short, shortValue[len(shortValue)-1], s.long, longValue[len(longValue)-1], ord.Price)
					s.holdOder = nil
				}
			}
		}
	}
}
