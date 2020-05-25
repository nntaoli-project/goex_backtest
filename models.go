package main

import (
	"github.com/nntaoli-project/goex"
	"time"
)

type DataConfig struct {
	Ex       string
	Pair     goex.CurrencyPair
	StarTime time.Time
	EndTime  time.Time
	Size     int //多少档深度数据
	UnGzip   bool
}

type ExchangeSimConfig struct {
	ExName               string
	TakerFee             float64
	MakerFee             float64
	SupportCurrencyPairs []goex.CurrencyPair
	Account              goex.Account
	BackTestStartTime    time.Time
	BackTestEndTime      time.Time
	UnGzip               bool //是否解压
}
