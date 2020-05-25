package main

import (
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"github.com/nntaoli-project/goex"
	"io"
	"log"
	"os"
	"sort"
	"time"
)

type DepthDataLoader struct {
	*DataConfig
	depths        []goex.Depth
	curDate       time.Time
	currTimestamp time.Time
	beginTime     time.Time
	Index         int
	Progress      float64       //已完成
	WaitTime      time.Duration //预计还需多久
}

const dataBaseDir = "data"

func NewDepthDataLoader(config DataConfig) *DepthDataLoader {
	loader := &DepthDataLoader{
		DataConfig: &config,
		Index:      -1,
		curDate:    config.StarTime,
		beginTime:  time.Now(),
	}
	loader.loadData(true)
	return loader
}

func (loader *DepthDataLoader) loadData(first bool) {
	loader.depths = loader.depths[:0]
	loader.Index = -1

	if loader.curDate.After(loader.EndTime) {
		return
	}

	fileName := fmt.Sprintf("%s/%s_%s_%s.csv", dataBaseDir, loader.Ex,
		loader.Pair.ToLower().ToSymbol(""), loader.curDate.Format("2006-01-02"))

	var (
		ioReader io.ReadCloser
	)

	if loader.UnGzip {
		fileName += ".gz"
		f, err := os.Open(fileName)
		if err != nil {
			log.Println(err)
			return
		}
		ioReader, err = gzip.NewReader(f)
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		f, err := os.Open(fileName)
		if err != nil {
			log.Println(err)
			return
		}
		ioReader = f
	}

	csvReader := csv.NewReader(ioReader)
	records, err := csvReader.ReadAll()
	if err != nil {
		log.Println(err)
		return
	}

	step := loader.Size * 2

	for _, r := range records {
		dep := goex.Depth{
			ContractType: "",
			Pair:         loader.Pair,
			UTime:        time.Unix(goex.ToInt64(r[0])/1000, goex.ToInt64(r[0])%1000),
		}

		for i := 1; i < step+1; i += 2 {
			dep.AskList = append(dep.AskList, goex.DepthRecord{
				Price:  goex.ToFloat64(r[i]),
				Amount: goex.ToFloat64(r[i+1]),
			})
		}

		for i := step + 1; i < 2*step+1; i += 2 {
			dep.BidList = append(dep.BidList, goex.DepthRecord{
				Price:  goex.ToFloat64(r[i]),
				Amount: goex.ToFloat64(r[i+1]),
			})
		}

		sort.Sort(sort.Reverse(dep.AskList))

		loader.depths = append(loader.depths, dep)
	}

	if first {
		loader.StarTime = loader.depths[0].UTime
	}

	loader.curDate = loader.curDate.AddDate(0, 0, 1)
}

func (loader *DepthDataLoader) Next() *goex.Depth {
	if len(loader.depths)-1 <= loader.Index {
		loader.loadData(false)
	}
	if len(loader.depths) == 0 {
		return nil //finished
	}
	loader.Index++
	return &loader.depths[loader.Index]
}

func (loader *DepthDataLoader) ComputeProgress() {
	remain := loader.EndTime.Sub(loader.currTimestamp)
	total := loader.EndTime.Sub(loader.StarTime)
	now := time.Now()
	finished := loader.currTimestamp.Sub(loader.StarTime)
	elapsed := now.Sub(loader.beginTime)
	loader.WaitTime = (elapsed / finished) * remain
	loader.Progress = float64(finished) / float64(total) * 100
}
