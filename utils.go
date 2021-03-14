package main

import (
	"encoding/json"
	"github.com/BurntSushi/toml"
	"github.com/nntaoli-project/goex"
	"time"
)

func DeepCopyStruct(source, target interface{}) {
	data, _ := json.Marshal(source)
	json.Unmarshal(data, target)
}

func LoadTomlConfig(tomlFile string) (ExchangeSimConfig, error) {
	var (
		simConfig  ExchangeSimConfig
		tomlConfig struct {
			ExName               string
			TakerFee             float64
			MakerFee             float64
			SupportCurrencyPairs []string
			QuoteCurrency        goex.Currency      `toml:"quote_currency"` //净值币种
			Account              map[string]float64 `toml:"accounts"`
			BackTestStartTime    time.Time
			BackTestEndTime      time.Time
			DepthSize            int  //回测多少档深度
			UnGzip               bool //是否解压
			BackTestDataType     BackTestDataType
		}
	)

	_, err := toml.DecodeFile(tomlFile, &tomlConfig)
	if err != nil {
		return simConfig, err
	}

	simConfig.ExName = tomlConfig.ExName
	simConfig.QuoteCurrency = tomlConfig.QuoteCurrency
	simConfig.TakerFee = tomlConfig.TakerFee
	simConfig.MakerFee = tomlConfig.MakerFee
	simConfig.DepthSize = tomlConfig.DepthSize
	simConfig.UnGzip = tomlConfig.UnGzip
	simConfig.BackTestEndTime = tomlConfig.BackTestEndTime
	simConfig.BackTestStartTime = tomlConfig.BackTestStartTime
	simConfig.BackTestData = tomlConfig.BackTestDataType

	for _, pair := range tomlConfig.SupportCurrencyPairs {
		simConfig.SupportCurrencyPairs = append(simConfig.SupportCurrencyPairs, goex.NewCurrencyPair2(pair))
	}

	simConfig.Account.SubAccounts = make(map[goex.Currency]goex.SubAccount, 1)
	for c, v := range tomlConfig.Account {
		currency := goex.NewCurrency(c, "")
		simConfig.Account.SubAccounts[currency] = goex.SubAccount{
			Currency: currency,
			Amount:   v,
		}
	}

	return simConfig, err
}
