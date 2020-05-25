package main

import (
	"context"
	"github.com/nntaoli-project/goex"
	"github.com/nntaoli-project/goex_backtest/strategies"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	log.Println("begin backtest")

	okexSim := NewExchangeSim(ExchangeSimConfig{
		ExName:               goex.BINANCE,
		TakerFee:             0.0045,
		MakerFee:             0.00018,
		SupportCurrencyPairs: []goex.CurrencyPair{goex.BTC_USDT},
		Account: goex.Account{
			SubAccounts: map[goex.Currency]goex.SubAccount{
				goex.BTC: goex.SubAccount{
					Currency: goex.BTC,
					Amount:   1,
				},
				goex.USDT: goex.SubAccount{
					Currency: goex.USDT,
					Amount:   10000,
				},
			},
		},
		BackTestStartTime: time.Date(2020, 03, 12, 0, 0, 0, 0, time.Local),
		BackTestEndTime:   time.Date(2020, 03, 12, 0, 0, 0, 0, time.Local),
		UnGzip:            false,
	})

	strategy := strategies.NewSampleStrategy(okexSim)
	ctx, cancel := context.WithCancel(context.Background())

	strategy.Main(ctx)

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

}
