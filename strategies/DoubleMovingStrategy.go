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
	pendingOrder *goexmodel.Order
	positionQty  float64
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
			// GetKline 内部已经用新K线撮合了挂单，这里结算并撤销未成交部分
			s.settlePendingOrder()
			for _, kline := range data {
				s.closePrices = append(s.closePrices, kline.Close)
			}
			if len(s.closePrices) < s.long || s.pendingOrder != nil {
				continue
			}
			// 均线基于最近 long 根K线计算
			window := s.closePrices[len(s.closePrices)-s.long:]
			long := talib.Ema(window, s.long)
			short := talib.Ema(window, s.short)
			price := data[0].Close

			if short[len(short)-1] > long[len(long)-1] {
				if s.positionQty <= 0 {
					s.placeOrder(goexmodel.Spot_Buy, 0.4, price)
				}
			} else if s.positionQty > 0 {
				qty := s.availableBaseQty()
				if qty <= 0 {
					s.positionQty = 0
					continue
				}
				s.placeOrder(goexmodel.Spot_Sell, qty, price)
			}
		}
	}
}

// settlePendingOrder 结算上一根K线挂出的限价单。限价单只在新的一根K线内尝试成交，
// 未成交或部分成交的剩余部分会被撤销，下一根K线再根据信号重新挂单。
func (s *DoubleMovingStrategy) settlePendingOrder() {
	if s.pendingOrder == nil {
		return
	}
	order, _, err := s.api.GetOrderInfo(s.pair, s.pendingOrder.Id)
	if err != nil {
		s.pendingOrder = nil
		return
	}

	filled := order.ExecutedQty > 0
	if filled {
		if order.Side == goexmodel.Spot_Buy {
			// 买入手续费扣的是基础币，持仓只记录到账后的净数量
			qty := order.ExecutedQty
			if order.FeeCcy == s.pair.BaseSymbol {
				qty -= order.Fee
			}
			s.positionQty += qty
		} else {
			s.positionQty -= order.ExecutedQty
			if s.positionQty < 0 {
				s.positionQty = 0
			}
		}
	}

	if order.Status == goexmodel.OrderStatus_Pending || order.Status == goexmodel.OrderStatus_PartFinished {
		if _, err := s.api.CancelOrder(s.pair, order.Id); err != nil {
			log.Printf("[撤单失败] ID:%s, 错误:%v", order.Id, err)
		}
	}
	s.pendingOrder = nil

	if !filled {
		log.Printf("[订单未成交] ID:%s", order.Id)
		return
	}

	s.api.AssetSnapshot()
	if order.Side == goexmodel.Spot_Buy {
		log.Printf("[开仓] 成交价:%f, 数量:%f", order.PriceAvg, order.ExecutedQty)
	} else {
		log.Printf("[平仓] 成交价:%f, 数量:%f", order.PriceAvg, order.ExecutedQty)
	}
}

func (s *DoubleMovingStrategy) placeOrder(side goexmodel.OrderSide, qty, price float64) {
	order, _, err := s.api.CreateOrder(s.pair, qty, price, side, goexmodel.OrderType_Limit)
	if err != nil {
		log.Printf("[下单失败] side:%s, 价格:%f, 错误:%v", side, price, err)
		return
	}
	s.pendingOrder = order
	if side == goexmodel.Spot_Buy {
		log.Printf("[开仓信号] 限价:%f", price)
	} else {
		log.Printf("[平仓信号] 限价:%f", price)
	}
}

func (s *DoubleMovingStrategy) availableBaseQty() float64 {
	qty := s.positionQty
	acc, _, err := s.api.GetAccount(s.pair.BaseSymbol)
	if err == nil {
		if a, ok := acc[s.pair.BaseSymbol]; ok && a.AvailableBalance < qty {
			qty = a.AvailableBalance
		}
	}
	return qty
}
