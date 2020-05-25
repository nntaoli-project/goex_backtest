package strategies

import (
	"context"
	"fmt"
	"github.com/nntaoli-project/goex"
	"log"
)

type SampleStragtegy struct {
	api goex.API
}

func NewSampleStrategy(api goex.API) *SampleStragtegy {
	return &SampleStragtegy{api: api}
}

func (s *SampleStragtegy) Main(ctx context.Context) {
	var (
		isFinishBuy  bool
		buyOrd       *goex.Order
		ord          *goex.Order
		isFinishSell bool
	)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			dep, err := s.api.GetDepth(1, goex.BTC_USDT)
			if err != nil && err.Error() == "depth data finished" {
				log.Println("finished")
				return
			}

			if ord == nil {
				log.Println("========= limit sell =======")
				log.Println(dep.AskList[len(dep.AskList)-1], ";", dep.BidList[0])
				ord, _ = s.api.LimitSell("0.01", fmt.Sprint(dep.AskList[len(dep.AskList)-1].Price), goex.BTC_USDT)
				log.Println(ord.Status, ord.OrderID2, ord.Amount, ord.DealAmount, ord.AvgPrice, ord.Fee)
				log.Println(s.api.GetAccount())
				isFinishSell = false
				log.Println("=========== end ===========")
			}

			if !isFinishSell && ord.Status == goex.ORDER_FINISH {
				log.Println("==== limit sell finished===")
				log.Println(s.api.GetAccount())
				log.Println(ord)
				log.Println(dep.AskList[len(dep.AskList)-1], ";", dep.BidList[0])
				isFinishSell = true
				log.Println("==== end====")
			}

			if buyOrd == nil {
				log.Println("====== limit buy ====")
				buyOrd, _ = s.api.LimitBuy("0.02", fmt.Sprintf("%.2f", dep.BidList[0].Price), goex.BTC_USDT)
				log.Println(buyOrd)
				log.Println(s.api.GetAccount())
				log.Println("====end===")
			}

			if !isFinishBuy && buyOrd.Status == goex.ORDER_FINISH {
				log.Println("=== limit buy finished ==== ")
				log.Println(s.api.GetAccount())
				log.Println(buyOrd)
				log.Println(dep.AskList[len(dep.AskList)-1], ";", dep.BidList[0])
				isFinishBuy = true
				log.Println("==== end====")
			}
		}
	}
}
