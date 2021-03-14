package sim

import (
	"encoding/csv"
	"fmt"
	"github.com/go-echarts/go-echarts/charts"
	"github.com/nntaoli-project/goex"
	"log"
	"os"
)

type BacktestStatistics struct {
	sims []*ExchangeSim
}

func NewBacktestStatistics(sims []*ExchangeSim) *BacktestStatistics {
	return &BacktestStatistics{
		sims: sims,
	}
}

func (s *BacktestStatistics) NetAssetReport() {
	lineChart := charts.NewLine()

	lineChart.SetGlobalOptions(
		charts.TitleOpts{
			Title:    "回测",
			Subtitle: "https://github.com/nntaoli-project/goex_backtest",
			//Top:      "20px",
			//Left:     "400px",
			//Bottom:   "150px",
		},
		charts.InitOpts{PageTitle: "净值", Width: "1080px"},
		charts.YAxisOpts{SplitLine: charts.SplitLineOpts{Show: true}, Scale: true},
	)

	for _, ex := range s.sims {
		assetSnapshotFile, err := os.Open(fmt.Sprintf("%s_asset_snapshot.csv", ex.GetExchangeName()))
		if err != nil {
			log.Printf("[ERROR] not found the %s  asset snapshot file ,error=%s", ex.GetExchangeName(), err)
			continue
		}
		csvR := csv.NewReader(assetSnapshotFile)
		records, err := csvR.ReadAll()
		if err != nil {
			log.Printf("[ERROR] read the %s asset snapshot file error=%s", ex.GetExchangeName(), err)
			continue
		}

		var (
			netAsset []float64
			xData    []int
		)
		for i, record := range records[1:] {
			curNetAsset := goex.ToFloat64(record[len(record)-1])
			netAsset = append(netAsset, curNetAsset)
			xData = append(xData, i)
		}

		lineChart.AddXAxis(xData)
		lineChart.AddYAxis("净值", netAsset,
			charts.MPNameTypeItem{Name: "最大值", Type: "max"},
			charts.MPNameTypeItem{Name: "最小值", Type: "min"},
			charts.MPStyleOpts{Label: charts.LabelTextOpts{Show: true}},
		)
	}

	netAssetF, _ := os.OpenFile("net_asset.html", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0744)
	lineChart.Render(netAssetF)
}

func (s *BacktestStatistics) OrderReport() {

}

func (s *BacktestStatistics) TaLibReport() {

}
