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
	QuoteCurrency        goex.Currency //净值币种
	Account              goex.Account
	BackTestStartTime    time.Time
	BackTestEndTime      time.Time
	DepthSize            int              //回测多少档深度
	UnGzip               bool             //是否解压
	BackTestData         BackTestDataType //回测数据类型
}

type BackTestDataType int

const (
	BackTestDataType_Depth BackTestDataType = iota + 1
	BackTestDataType_KLine
)
