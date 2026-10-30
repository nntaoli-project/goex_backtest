package util

import (
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	goexmodel "github.com/nntaoli-project/goex/v2/model"
	"github.com/nntaoli-project/goex_backtest/model"
	"strings"
	"time"
)

func DeepCopyStruct(source, target interface{}) {
	data, _ := json.Marshal(source)
	json.Unmarshal(data, target)
}

func LoadTomlConfig(tomlFile string) (model.ExchangeSimConfig, error) {
	var (
		simConfig  model.ExchangeSimConfig
		tomlConfig struct {
			ExName               string
			TakerFee             float64
			MakerFee             float64
			SupportCurrencyPairs []string
			QuoteCurrency        struct {
				Symbol string `toml:"symbol"`
			} `toml:"quote_currency"`
			Account           map[string]float64 `toml:"accounts"`
			BackTestStartTime time.Time
			BackTestEndTime   time.Time
			DepthSize         int  //回测多少档深度
			UnGzip            bool //是否解压
			BackTestDataType  model.BackTestDataType
		}
	)

	_, err := toml.DecodeFile(tomlFile, &tomlConfig)
	if err != nil {
		return simConfig, err
	}

	simConfig.ExName = tomlConfig.ExName
	simConfig.QuoteCurrency = strings.ToUpper(tomlConfig.QuoteCurrency.Symbol)
	simConfig.TakerFee = tomlConfig.TakerFee
	simConfig.MakerFee = tomlConfig.MakerFee
	simConfig.DepthSize = tomlConfig.DepthSize
	simConfig.UnGzip = tomlConfig.UnGzip
	simConfig.BackTestEndTime = tomlConfig.BackTestEndTime
	simConfig.BackTestStartTime = tomlConfig.BackTestStartTime
	simConfig.BackTestData = tomlConfig.BackTestDataType

	for _, pair := range tomlConfig.SupportCurrencyPairs {
		parts := strings.Split(strings.ToUpper(pair), "_")
		if len(parts) != 2 {
			return simConfig, fmt.Errorf("invalid currency pair %q", pair)
		}
		simConfig.SupportCurrencyPairs = append(simConfig.SupportCurrencyPairs, goexmodel.CurrencyPair{
			Symbol: parts[0] + parts[1], BaseSymbol: parts[0], QuoteSymbol: parts[1],
		})
	}

	simConfig.Accounts = make(map[string]goexmodel.Account, len(tomlConfig.Account))
	for c, v := range tomlConfig.Account {
		coin := strings.ToUpper(c)
		simConfig.Accounts[coin] = goexmodel.Account{
			Coin: coin, Balance: v, AvailableBalance: v,
		}
	}

	return simConfig, err
}
