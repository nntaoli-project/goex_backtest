package strategies

import (
	"context"
	"github.com/markcheno/go-talib"
	goexmodel "github.com/nntaoli-project/goex/v2/model"
	"github.com/nntaoli-project/goex_backtest/sim"
	"log"
)

type DoubleMovingStrategy struct {
	api          *sim.ExchangeSim
	pair         goexmodel.CurrencyPair
	period       goexmodel.KlinePeriod
	long, short  int
	holdOrder    *goexmodel.Order
	pendingOrder *goexmodel.Order
	closePrices  []float64
}

func NewDoubleMovingStrategy(api *sim.ExchangeSim, period goexmodel.KlinePeriod, long, short int, pair goexmodel.CurrencyPair) *DoubleMovingStrategy {
	return &DoubleMovingStrategy{api: api, pair: pair, period: period, long: long, short: short}
}
func (s *DoubleMovingStrategy) Main(ctx context.Context) {
	defer s.api.AssetSnapshot()
	
	for {
		select {
		case <-ctx.Done():
			return
		default:
			data, _, err := s.api.GetKline(s.pair, s.period)
			if err != nil {
				return
			}
			s.syncPendingOrder()
			for _, kline := range data {
				s.closePrices = append(s.closePrices, kline.Close)
			}
			if len(s.closePrices) < s.long {
				continue
			}
			long := talib.Ema(s.closePrices, s.long)
			short := talib.Ema(s.closePrices, s.short)
			if s.pendingOrder != nil {
				continue
			}
			if short[len(short)-1] > long[len(long)-1] {
				if s.holdOrder == nil {
					order, _, err := s.api.CreateOrder(s.pair, 0.4, 0, goexmodel.Spot_Buy, goexmodel.OrderType_Market)
					if err == nil {
						s.pendingOrder = order
						log.Printf("[开仓信号] 收盘价:%f", data[0].Close)
					}
				}
			} else if s.holdOrder != nil {
				// 买入手续费扣的是基础币，卖出时只能卖到账后的净数量
				qty := s.holdOrder.ExecutedQty
				if s.holdOrder.FeeCcy == s.pair.BaseSymbol {
					qty -= s.holdOrder.Fee
				}
				if qty <= 0 {
					s.holdOrder = nil
					continue
				}
				order, _, err := s.api.CreateOrder(s.pair, qty, 0, goexmodel.Spot_Sell, goexmodel.OrderType_Market)
				if err == nil {
					s.pendingOrder = order
					log.Printf("[平仓信号] 收盘价:%f", data[0].Close)
				}
			}
		}
	}
}

func (s *DoubleMovingStrategy) syncPendingOrder() {
	if s.pendingOrder == nil {
		return
	}
	order, _, err := s.api.GetOrderInfo(s.pair, s.pendingOrder.Id)
	if err != nil || order.Status == goexmodel.OrderStatus_Pending || order.Status == goexmodel.OrderStatus_PartFinished {
		return
	}
	s.pendingOrder = nil
	if order.Status != goexmodel.OrderStatus_Finished {
		log.Printf("[订单取消] ID:%s", order.Id)
		return
	}
	if order.Side == goexmodel.Spot_Buy {
		s.holdOrder = order
		log.Printf("[开仓] 价格:%f", order.PriceAvg)
	} else {
		s.holdOrder = nil
		log.Printf("[平仓] 价格:%f", order.PriceAvg)
	}
	s.api.AssetSnapshot()
}
