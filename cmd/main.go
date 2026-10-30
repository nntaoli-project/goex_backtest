package main

import (
	"context"
	goexmodel "github.com/nntaoli-project/goex/v2/model"
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
	sim := sim2.NewExchangeSimWithTomlConfig("huobi.pro")

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

	strategy := strategies.NewDoubleMovingStrategy(sim, goexmodel.Kline_1min, 600, 150, goexmodel.CurrencyPair{Symbol: "BTCUSDT", BaseSymbol: "BTC", QuoteSymbol: "USDT"})
	strategy.Main(ctx)

	backtestStatistics.NetAssetReport()
	backtestStatistics.OrderReport()
	backtestStatistics.TaLibReport()

	log.Println("###### end backtest , elapsed ", time.Now().Sub(beginT), "######")
}
