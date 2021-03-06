package main

import (
	"context"
	"github.com/nntaoli-project/goex"
	sim2 "github.com/nntaoli-project/goex_backtest/sim"
	"github.com/nntaoli-project/goex_backtest/strategies"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	log.Println("###### begin backtest ######")
	beginT := time.Now()
	//sim := NewExchangeSim(ExchangeSimConfig{
	//	ExName:               goex.BINANCE,
	//	TakerFee:             0.00025,
	//	MakerFee:             -0.00015,
	//	QuoteCurrency:        goex.USDT,
	//	SupportCurrencyPairs: []goex.CurrencyPair{goex.BTC_USDT}, // CurrencyB 同一次回测只支持一种交易市场的交易对,为了净值统计
	//	Account: goex.Account{
	//		SubAccounts: map[goex.Currency]goex.SubAccount{
	//			goex.BTC: {
	//				Currency: goex.BTC,
	//				Amount:   0,
	//			},
	//			goex.USDT: {
	//				Currency: goex.USDT,
	//				Amount:   10000,
	//			},
	//		},
	//	},
	//	BackTestStartTime: time.Date(2020, 03, 12, 0, 0, 0, 0, time.Local),
	//	BackTestEndTime:   time.Date(2020, 03, 12, 0, 0, 0, 0, time.Local),
	//	DepthSize:         20, //深度挡数
	//	UnGzip:            false,
	//})

	sim := sim2.NewExchangeSimWithTomlConfig(goex.HUOBI_PRO)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, os.Kill)
		for {
			select {
			case <-sig:
				cancel()
				return
			}
		}
	}()

	backtestStatistics := sim2.NewBacktestStatistics([]*sim2.ExchangeSim{sim})

	strategy := strategies.NewDoubleMovingStrategy(sim, goex.KLINE_PERIOD_1MIN, 600, 150, goex.BTC_USDT)
	strategy.Main(ctx)

	backtestStatistics.NetAssetReport()
	backtestStatistics.OrderReport()
	backtestStatistics.TaLibReport()

	log.Println("###### end backtest , elapsed ", time.Now().Sub(beginT), "######")
}
