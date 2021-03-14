package loader

import (
	"compress/gzip"
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/nntaoli-project/goex"
	"github.com/nntaoli-project/goex_backtest/model"
	"github.com/spf13/cast"
	"io"
	"log"
	"os"
	"time"
)

type KlineDatas struct {
	Index       int
	CurrentDate time.Time
	Data        []goex.Kline
}

type KLineDataLoader struct {
	model.DataConfig
	data map[goex.CurrencyPair]map[goex.KlinePeriod]*KlineDatas
}

func NewKLineDataLoader(c model.DataConfig) *KLineDataLoader {
	loader := &KLineDataLoader{
		DataConfig: c,
		data:       make(map[goex.CurrencyPair]map[goex.KlinePeriod]*KlineDatas, 2),
	}
	return loader
}

func (loader *KLineDataLoader) load(pair goex.CurrencyPair, period goex.KlinePeriod) {
	if loader.data[pair] == nil {
		loader.data[pair] = make(map[goex.KlinePeriod]*KlineDatas, 2)
	}

	data := loader.data[pair][period]
	if data == nil {
		loader.data[pair][period] = new(KlineDatas)
		loader.data[pair][period].CurrentDate = loader.StarTime
		data = loader.data[pair][period]
	}

	if data.CurrentDate.After(loader.EndTime) {
		return
	}

	dateStr := data.CurrentDate.Format("2006-01-02")
	fileName := fmt.Sprintf("%s_kline_%s_%s_%s",
		loader.Ex, pair.ToLower().ToSymbol(""), loader.adaptKlinePeriod(period), dateStr)

	if loader.UnGzip {
		fileName += ".gz"
	} else {
		fileName += ".csv"
	}

	log.Printf("###### begin load the %s ######", fileName)

	var reader io.Reader

	f, err := os.Open(fmt.Sprintf("data/%s", fileName))
	if err != nil {
		log.Println("open file error", err)
		return
	}

	if loader.UnGzip {
		r, err := gzip.NewReader(f)
		if err != nil {
			log.Println("gzip read error", err)
			return
		}
		reader = r
	} else {
		reader = f
	}

	csvReader := csv.NewReader(reader)
	records, err := csvReader.ReadAll()
	if err != nil {
		log.Println(err)
		return
	}

	for _, line := range records {
		kline := goex.Kline{
			Pair:      pair,
			Timestamp: cast.ToInt64(line[0]),
			Open:      cast.ToFloat64(line[3]),
			Close:     cast.ToFloat64(line[4]),
			High:      cast.ToFloat64(line[1]),
			Low:       cast.ToFloat64(line[2]),
			Vol:       cast.ToFloat64(line[5]),
		}
		data.Data = append(data.Data, kline)
	}

	log.Printf("###### end load , current size %d ######", len(data.Data))

	data.CurrentDate = data.CurrentDate.AddDate(0, 0, 1)
}

func (loader *KLineDataLoader) Next(pair goex.CurrencyPair, period goex.KlinePeriod, size int) (klineData []goex.Kline, err error) {
	//huobi.pro_kline_btcusdt_1min_2020-10-22.csv
	data := loader.data[pair][period]
	if data == nil || len(data.Data) < size+data.Index {
		loader.load(pair, period)
		data = loader.data[pair][period]
		if data == nil || len(data.Data) < size+data.Index {
			return nil, errors.New("no data")
		}
	}

	defer func() {
		data.Index = data.Index + size
	}()

	tmp := data.Data[data.Index : data.Index+size]

	for i := len(tmp) - 1; i >= 0; i-- {
		klineData = append(klineData, tmp[i])
	}

	return klineData, nil
}

func (loader *KLineDataLoader) adaptKlinePeriod(period goex.KlinePeriod) string {
	switch period {
	case goex.KLINE_PERIOD_1DAY:
		return "1d"
	case goex.KLINE_PERIOD_4H:
		return "1h"
	case goex.KLINE_PERIOD_1H:
		return "1h"
	case goex.KLINE_PERIOD_30MIN:
		return "30min"
	case goex.KLINE_PERIOD_15MIN:
		return "15min"
	case goex.KLINE_PERIOD_1MIN:
		return "1min"
	default:
		return "1min"
	}
}
