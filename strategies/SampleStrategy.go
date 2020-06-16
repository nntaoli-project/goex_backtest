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
		buyOrd     *goex.Order
		ord        *goex.Order
		amount     = 0.01
		sellCount  = 0
		buyCount   = 0
		sellOffset = 0.2
		buyOffset  = 0.2
	)

	defer func() {
		log.Println("=========== 买:", buyCount, "==========")
		log.Println("=========== 卖:", sellCount, "==========")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			dep, err := s.api.GetDepth(1, goex.BTC_USDT)
			if err != nil && err.Error() == "depth data finished" {
				log.Println("========== finished ==========")
				return
			}

			askItem := dep.AskList[len(dep.AskList)-1]
			bidItem := dep.BidList[0]
			if askItem.Price <= 0 || askItem.Amount <= 0 || bidItem.Price <= 0 || bidItem.Amount <= 0 {
				//	log.Println(dep)
				continue
			}

			acc, _ := s.api.GetAccount()
			btcAcc := acc.SubAccounts[goex.BTC]
			usdtAcc := acc.SubAccounts[goex.USDT]

			if ord == nil {
				if btcAcc.Amount < amount {
					goto buy
				}

				ord, err = s.api.LimitSell(fmt.Sprint(amount), fmt.Sprint(dep.AskList[len(dep.AskList)-1].Price), goex.BTC_USDT)
				if err != nil {
					log.Println("[ERROR] ", err)
				} else {
					log.Println(ord)
				}
			} else {
				_ord, err := s.api.GetOneOrder(ord.OrderID2, ord.Currency)
				if err != nil {
					panic(err)
				}
				ord = _ord
			}

			if ord != nil && ord.Status == goex.ORDER_FINISH {
				log.Println(ord)
				sellCount++
				ord = nil //reset
			} else if ord != nil {
				offset := (ord.Price - dep.BidList[0].Price) / ord.Price * 100
				if offset >= sellOffset {
					_, err := s.api.CancelOrder(ord.OrderID2, goex.BTC_USDT)
					if err != nil {
						log.Println("[ERROR] cancel error=", err)
						log.Println(s.api.GetOneOrder(ord.OrderID2, goex.BTC_USDT))
					}
					ord = nil
				}
			}

		buy:
			if buyOrd == nil {
				if usdtAcc.Amount < dep.BidList[0].Price*amount {
					goto end
				}

				buyOrd, err = s.api.LimitBuy(fmt.Sprint(amount), fmt.Sprintf("%.2f", dep.BidList[0].Price), goex.BTC_USDT)
				if err != nil {
					log.Println("[ERROR] ", err)
				} else {
					log.Println(buyOrd)
				}
			} else {
				_ord, err := s.api.GetOneOrder(buyOrd.OrderID2, buyOrd.Currency)
				if err != nil {
					panic(err)
				}
				buyOrd = _ord
			}

			if buyOrd != nil && buyOrd.Status == goex.ORDER_FINISH {
				log.Println(buyOrd)
				buyCount++
				buyOrd = nil
			} else if buyOrd != nil {
				offset := (dep.AskList[len(dep.AskList)-1].Price - buyOrd.Price) / buyOrd.Price * 100
				if offset >= buyOffset {
					log.Println("[cancel] ", buyOrd.OrderID2)
					_, err := s.api.CancelOrder(buyOrd.OrderID2, goex.BTC_USDT)
					if err != nil {
						log.Println("[ERROR] cancel error=", err)
						log.Println(s.api.GetOneOrder(buyOrd.OrderID2, goex.BTC_USDT))
					}
					buyOrd = nil
				}
			}

		end:
		}
	}
}
